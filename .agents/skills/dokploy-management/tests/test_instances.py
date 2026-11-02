"""Named-instance regression tests using synthetic credentials and no server."""

import importlib.util
import io
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from dataclasses import replace
from pathlib import Path
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "dokploy_instance.py"
SPEC = importlib.util.spec_from_file_location("dokploy_instance", SCRIPT)
helper = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = helper
SPEC.loader.exec_module(helper)
REAL_CLI = shutil.which("dokploy-cli")


class InstanceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.config = self.root / "instances.toml"
        self.cache = self.root / "cache"
        self.environment = patch.dict(
            os.environ,
            {
                "XDG_CONFIG_HOME": str(self.root / "config"),
                "XDG_CACHE_HOME": str(self.cache),
                "DOKPLOY_URL": "https://legacy.example.com",
                "DOKPLOY_API_KEY": "legacy-secret",
                "FORGE_KEY": "forge-secret",
                "MIND_KEY": "mind-secret",
            },
        )
        self.environment.start()
        self.addCleanup(self.environment.stop)
        self.key = self.root / "secrets" / "home-server.key"
        self.key.parent.mkdir(mode=0o700)
        self.key.write_text("home-secret\n")
        self.key.chmod(0o600)
        self.config.write_text(
            '[instances.home-server]\nurl = "https://home.example.com/"\n'
            'ssh_host = "home-server"\napi_key_file = "secrets/home-server.key"\n\n'
            '[instances.quantforge]\nurl = "https://forge.example.com"\n'
            'ssh_host = "quantforge"\napi_key_env = "FORGE_KEY"\n\n'
            '[instances.quantmind]\nurl = "https://mind.example.com"\n'
            'ssh_host = "quantmind"\napi_key_env = "MIND_KEY"\n'
        )
        self.instances = helper.load_instances(self.config)

    def invoke(self, *args):
        stdout, stderr = io.StringIO(), io.StringIO()
        with redirect_stdout(stdout), redirect_stderr(stderr):
            result = helper.main(["--config", str(self.config), *args])
        return result, stdout.getvalue(), stderr.getvalue()

    def test_loads_three_aliases_and_relative_credential_reference(self):
        self.assertEqual(
            set(self.instances), {"home-server", "quantforge", "quantmind"}
        )
        self.assertEqual(self.instances["home-server"].url, "https://home.example.com")
        self.assertEqual(self.instances["home-server"].api_key_file, self.key)
        self.assertEqual(
            helper.read_api_key(self.instances["home-server"]), "home-secret"
        )

    def test_listing_reads_no_secrets(self):
        self.key.unlink()
        with patch.object(
            helper,
            "read_api_key",
            side_effect=AssertionError("Must not read credentials"),
        ):
            code, out, err = self.invoke("list", "--json")
        self.assertEqual(code, 0)
        self.assertEqual(err, "")
        rows = json.loads(out)["data"]
        self.assertEqual(
            [row["alias"] for row in rows], ["home-server", "quantforge", "quantmind"]
        )
        self.assertNotIn("secret", out)
        self.assertNotIn(str(self.key), out)

    def test_missing_config_and_unknown_alias_never_fall_back(self):
        with patch.object(helper.subprocess, "run") as child:
            code, out, err = self.invoke("run", "unknown", "--", "service", "list")
            self.assertEqual(code, 2)
            self.assertEqual(out, "")
            self.assertIn("Unknown instance alias", err)
            self.config.unlink()
            code, out, err = self.invoke("list")
            self.assertEqual(code, 2)
            self.assertIn("config not found", err)
            child.assert_not_called()

    def test_cli_environment_argv_and_exit_code_are_isolated(self):
        parent = dict(os.environ)
        with (
            patch.object(helper, "executable", return_value="/fake/cli"),
            patch.object(
                helper.subprocess,
                "run",
                return_value=subprocess.CompletedProcess([], 17),
            ) as child,
        ):
            code, out, err = self.invoke(
                "run", "home-server", "--", "--json", "service", "list"
            )
        self.assertEqual(code, 17)
        self.assertEqual(out, "")
        self.assertIn("[instance home-server]", err)
        self.assertNotIn("secret", err)
        argv = child.call_args.args[0]
        env = child.call_args.kwargs["env"]
        self.assertEqual(
            argv,
            ["/fake/cli", "--json", "service", "list"],
        )
        self.assertNotIn("home-secret", " ".join(argv))
        self.assertEqual(env["DOKPLOY_URL"], "https://home.example.com")
        self.assertEqual(env["DOKPLOY_API_KEY"], "home-secret")
        self.assertNotIn("FORGE_KEY", env)
        self.assertNotIn("MIND_KEY", env)
        self.assertEqual(child.call_args.kwargs["umask"], 0o077)
        self.assertEqual(dict(os.environ), parent)

    def test_key_file_requires_private_permissions_and_nonempty_single_token(self):
        for mode in (0o644, 0o640, 0o666):
            self.key.chmod(mode)
            with (
                self.subTest(mode=mode),
                self.assertRaisesRegex(helper.ConfigError, "mode 600"),
            ):
                helper.read_api_key(self.instances["home-server"])
        self.key.chmod(0o600)
        for value in ("", " \n", "two words", "line\nbreak", "key\x00suffix"):
            self.key.write_text(value)
            with self.subTest(value=value), self.assertRaises(helper.ConfigError):
                helper.read_api_key(self.instances["home-server"])
        self.key.write_text("home-secret\n")
        self.key.chmod(0o400)
        self.assertEqual(
            helper.read_api_key(self.instances["home-server"]), "home-secret"
        )

    def test_missing_env_key_does_not_fall_back_to_legacy_key(self):
        del os.environ["FORGE_KEY"]
        with patch.object(helper.subprocess, "run") as child:
            code, out, err = self.invoke("run", "quantforge", "--", "service", "list")
        self.assertEqual(code, 2)
        self.assertIn("no fallback", err)
        self.assertNotIn("legacy-secret", out + err)
        child.assert_not_called()

    def test_rejects_url_overrides_and_unknown_root_options(self):
        invalid = [
            [],
            ["--json"],
            ["--timeout", "3"],
            ["--url", "https://other.example.com", "service", "list"],
            ["--url=https://other.example.com", "service", "list"],
            ["service", "list", "--url", "https://other.example.com"],
            ["--dry-run", "service", "list"],
            ["--", "service", "list"],
        ]
        for args in invalid:
            with self.subTest(args=args), self.assertRaises(helper.ConfigError):
                helper.validate_cli_args(args)
        for args in (
            ["--json", "service", "list"],
            ["--timeout", "5", "doctor"],
            ["--json", "--show-secrets", "--timeout=3", "service", "show", "compose", "c1"],
            ["--help"],
            ["--version"],
            ["service", "deploy", "compose", "c1", "--yes", "--dry-run"],
            ["api", "call", "compose.one", "-p", "composeId=--json"],
        ):
            helper.validate_cli_args(args)

    def test_config_validation_never_prints_supplied_secret_values(self):
        invalid = [
            '[instances."../other"]\nurl="https://ok.example.com"\napi_key_env="KEY"',
            '[instances.prod]\nurl="https://user:secret-marker@ok.example.com"\napi_key_env="KEY"',
            '[instances.prod]\nurl="https://ok.example.com?key=secret-marker"\napi_key_env="KEY"',
            '[instances.prod]\nurl="ftp://ok.example.com"\napi_key_env="KEY"',
            '[instances.prod]\nurl="https://ok.example.com:99999"\napi_key_env="KEY"',
            '[instances.prod]\nurl="https://ok.example.com"\napi_key="secret-marker"',
            '[instances.prod]\nurl="https://ok.example.com"\napi_key_env="KEY"\napi_key_file="a"',
            '[instances.prod]\nurl="https://ok.example.com"\napi_key_env="KEY"\nssh_host="-oProxyCommand=secret-marker"',
            "broken TOML secret-marker",
        ]
        for text in invalid:
            self.config.write_text(text)
            with self.subTest(text=text):
                code, out, err = self.invoke("list")
                self.assertEqual(code, 2)
                self.assertNotIn("secret-marker", out + err)

    def test_ssh_preserves_alias_and_quotes_metacharacters_without_api_secrets(self):
        self.key.unlink()
        with (
            patch.object(helper, "executable", return_value="/fake/ssh"),
            patch.object(
                helper.subprocess,
                "run",
                return_value=subprocess.CompletedProcess([], 23),
            ) as child,
        ):
            code, out, err = self.invoke(
                "ssh", "home-server", "--", "printf", "%s", "a; touch /tmp/no"
            )
        self.assertEqual(code, 23)
        self.assertEqual(out, "")
        self.assertIn("SSH home-server", err)
        self.assertEqual(
            child.call_args.args[0],
            [
                "/fake/ssh",
                "-o",
                "BatchMode=yes",
                "-o",
                "StrictHostKeyChecking=yes",
                "--",
                "home-server",
                "printf %s 'a; touch /tmp/no'",
            ],
        )
        for name in (
            "DOKPLOY_API_KEY",
            "DOKPLOY_URL",
            "FORGE_KEY",
            "MIND_KEY",
        ):
            self.assertNotIn(name, child.call_args.kwargs["env"])

    def test_ssh_requires_explicit_command_and_host(self):
        instances = self.instances
        with self.assertRaises(helper.ConfigError):
            helper.run_ssh(instances["home-server"], instances, [])
        with self.assertRaises(helper.ConfigError):
            helper.run_ssh(
                replace(instances["home-server"], ssh_host=None), instances, ["true"]
            )

    def test_signal_exit_code_is_preserved_as_shell_status(self):
        with (
            patch.object(helper, "executable", return_value="/fake/cli"),
            patch.object(
                helper.subprocess,
                "run",
                return_value=subprocess.CompletedProcess([], -15),
            ),
        ):
            code, _, _ = self.invoke("run", "quantmind", "--", "doctor")
        self.assertEqual(code, 143)

    def test_xdg_config_default(self):
        self.assertEqual(
            helper.default_config_path(),
            self.root / "config" / "dokploy-cli" / "instances.toml",
        )

    @unittest.skipUnless(REAL_CLI, "dokploy-cli is not installed")
    def test_actual_cli_receives_instance_url_without_network(self):
        # --dry-run validates against the bundled registry and sends nothing.
        for alias in self.instances:
            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT),
                    "--config",
                    str(self.config),
                    "run",
                    alias,
                    "--",
                    "--json",
                    "api",
                    "call",
                    "project.create",
                    "-p",
                    f"name={alias}",
                    "--dry-run",
                ],
                check=False,
                capture_output=True,
                text=True,
                timeout=30,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            envelope = json.loads(result.stdout)
            self.assertTrue(envelope["ok"])
            self.assertEqual(envelope["data"]["body"], {"name": alias})
            self.assertIn(f"[instance {alias}] {self.instances[alias].url}", result.stderr)
            self.assertNotIn("secret", result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
