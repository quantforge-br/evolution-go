"""Named-instance regression tests using synthetic credentials and local state."""

import importlib.util
import io
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import threading
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
REAL_CLI = shutil.which("cli-anything-dokploy")


class InstanceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.config = self.root / "instances.toml"
        self.state = self.root / "state"
        self.environment = patch.dict(
            os.environ,
            {
                "XDG_CONFIG_HOME": str(self.root / "config"),
                "XDG_STATE_HOME": str(self.state),
                "DOKPLOY_URL": "https://legacy.example.com",
                "DOKPLOY_API_KEY": "legacy-secret",
                "CLI_ANYTHING_DOKPLOY_SESSION": str(self.root / "legacy.json"),
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

    def state_path(self, alias):
        return (
            self.state / "cli-anything-dokploy" / "instances" / alias / "session.json"
        )

    def test_loads_three_aliases_and_relative_credential_reference(self):
        self.assertEqual(
            set(self.instances), {"home-server", "quantforge", "quantmind"}
        )
        self.assertEqual(self.instances["home-server"].url, "https://home.example.com")
        self.assertEqual(self.instances["home-server"].api_key_file, self.key)
        self.assertEqual(
            helper.read_api_key(self.instances["home-server"]), "home-secret"
        )

    def test_listing_reads_no_secrets_and_creates_no_state(self):
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
        self.assertFalse(self.state.exists())

    def test_missing_config_and_unknown_alias_never_fall_back(self):
        with patch.object(helper.subprocess, "run") as child:
            code, out, err = self.invoke("run", "unknown", "--", "services", "list")
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
                "run", "home-server", "--", "--json", "services", "list"
            )
        self.assertEqual(code, 17)
        self.assertEqual(out, "")
        self.assertIn("[instance home-server]", err)
        self.assertNotIn("secret", err)
        argv = child.call_args.args[0]
        env = child.call_args.kwargs["env"]
        self.assertEqual(
            argv,
            [
                "/fake/cli",
                "--session",
                str(self.state_path("home-server")),
                "--json",
                "services",
                "list",
            ],
        )
        self.assertNotIn("home-secret", " ".join(argv))
        self.assertEqual(env["DOKPLOY_URL"], "https://home.example.com")
        self.assertEqual(env["DOKPLOY_API_KEY"], "home-secret")
        self.assertEqual(
            env["CLI_ANYTHING_DOKPLOY_SESSION"], str(self.state_path("home-server"))
        )
        self.assertNotIn("FORGE_KEY", env)
        self.assertNotIn("MIND_KEY", env)
        self.assertEqual(child.call_args.kwargs["umask"], 0o077)
        self.assertEqual(dict(os.environ), parent)

    def test_aliases_keep_targets_staged_data_history_undo_and_previews_separate(self):
        for alias, instance in self.instances.items():
            with helper.bound_session(instance) as path:
                data = json.loads(path.read_text())
                data.update(
                    target={"compose_id": alias},
                    staged={"env": {alias: {"COLOR": alias}}},
                    undo=[{"kind": "target", "alias": alias}],
                    history=[{"alias": alias}],
                )
                path.write_text(json.dumps(data))
                previews = path.parent / "previews"
                previews.mkdir()
                (previews / "marker").write_text(alias)
        for alias, instance in self.instances.items():
            with helper.bound_session(instance) as path:
                data = json.loads(path.read_text())
                self.assertEqual(data["target"]["compose_id"], alias)
                self.assertEqual(data["staged"]["env"][alias]["COLOR"], alias)
                self.assertEqual(data["history"], [{"alias": alias}])
                self.assertEqual(data["undo"][0]["alias"], alias)
                self.assertEqual(
                    (path.parent / "previews" / "marker").read_text(), alias
                )
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
                self.assertEqual(stat.S_IMODE(path.parent.stat().st_mode), 0o700)
                self.assertNotIn("secret", path.read_text())

    def test_changed_url_and_corrupt_or_unbound_sessions_fail_closed(self):
        instance = self.instances["home-server"]
        with helper.bound_session(instance) as path:
            original = path.read_text()
        with (
            self.assertRaisesRegex(helper.ConfigError, "Session URL differs"),
            helper.bound_session(
                replace(instance, url="https://replacement.example.com")
            ),
        ):
            self.fail("Mismatched session was opened")
        self.assertEqual(path.read_text(), original)
        for corrupt in ("{", "[]", "null", "{}", '{"connection": null}'):
            path.write_text(corrupt)
            with (
                self.subTest(corrupt=corrupt),
                self.assertRaisesRegex(helper.ConfigError, "Invalid or unbound"),
                helper.bound_session(instance),
            ):
                self.fail("Invalid session was accepted")
            self.assertEqual(path.read_text(), corrupt)

    def test_repeated_runs_do_not_create_connection_undo_entries(self):
        instance = self.instances["home-server"]
        for _ in range(3):
            with helper.bound_session(instance) as path:
                self.assertNotIn("undo", json.loads(path.read_text()))

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
            code, out, err = self.invoke("run", "quantforge", "--", "services", "list")
        self.assertEqual(code, 2)
        self.assertIn("no fallback", err)
        self.assertNotIn("legacy-secret", out + err)
        child.assert_not_called()

    def test_rejects_routing_overrides_repl_and_connection_commands(self):
        invalid = [
            [],
            ["--json"],
            ["repl"],
            ["--timeout", "3", "repl"],
            ["--timeout=3", "repl"],
            ["session", "connect"],
            ["session", "disconnect"],
            ["--url", "https://other.example.com", "services", "list"],
            ["--session=other.json", "services", "list"],
            ["--project", "other.json", "services", "list"],
            ["--json", "repl"],
            ["--", "repl"],
        ]
        for args in invalid:
            with self.subTest(args=args), self.assertRaises(helper.ConfigError):
                helper.validate_cli_args(args)
        for args in (
            ["--json", "services", "list"],
            ["--timeout", "5", "session", "status"],
            ["--dry-run", "--timeout=3", "env", "staged"],
            ["--help"],
            ["api", "call", "compose", "one", "--param", "name=repl"],
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

    def test_session_and_alias_directory_symlinks_are_refused(self):
        path = self.state_path("home-server")
        path.parent.mkdir(parents=True)
        victim = self.root / "victim.json"
        victim.write_text('{"connection":{"url":"https://home.example.com"}}')
        path.symlink_to(victim)
        with (
            self.assertRaisesRegex(helper.ConfigError, "symbolic link"),
            helper.bound_session(self.instances["home-server"]),
        ):
            self.fail("Symlink was accepted")
        path.unlink()
        (path.parent / ".lock").unlink()
        path.parent.rmdir()
        path.parent.symlink_to(self.root / "secrets", target_is_directory=True)
        with (
            self.assertRaisesRegex(helper.ConfigError, "symbolic link"),
            helper.bound_session(self.instances["home-server"]),
        ):
            self.fail("Symlink directory was accepted")

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
            "CLI_ANYTHING_DOKPLOY_SESSION",
            "FORGE_KEY",
            "MIND_KEY",
        ):
            self.assertNotIn(name, child.call_args.kwargs["env"])
        self.assertFalse(self.state.exists())

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
            code, _, _ = self.invoke("run", "quantmind", "--", "session", "status")
        self.assertEqual(code, 143)

    def test_xdg_defaults(self):
        self.assertEqual(
            helper.default_config_path(),
            self.root / "config" / "cli-anything-dokploy" / "instances.toml",
        )
        self.assertEqual(
            helper.state_root(), self.state / "cli-anything-dokploy" / "instances"
        )

    def test_same_alias_lock_covers_child_lifetime(self):
        instance = self.instances["home-server"]
        started, entered = threading.Event(), threading.Event()
        errors = []

        def second():
            started.set()
            try:
                with helper.bound_session(instance):
                    entered.set()
            except (helper.ConfigError, OSError) as error:
                errors.append(error)

        with helper.bound_session(instance):
            thread = threading.Thread(target=second)
            thread.start()
            self.assertTrue(started.wait(2))
            self.assertFalse(entered.wait(0.1))
            with helper.bound_session(self.instances["quantforge"]):
                pass
        thread.join(2)
        self.assertFalse(thread.is_alive())
        self.assertTrue(entered.is_set())
        self.assertFalse(errors)

    @unittest.skipUnless(REAL_CLI, "cli-anything-dokploy is not installed")
    def test_actual_cli_local_session_and_stdout_integration(self):
        for alias, instance in self.instances.items():
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
                    "session",
                    "status",
                ],
                check=False,
                capture_output=True,
                text=True,
                timeout=15,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            envelope = json.loads(result.stdout)
            self.assertTrue(envelope["success"])
            self.assertEqual(envelope["data"]["url"], instance.url)
            self.assertEqual(envelope["data"]["path"], str(self.state_path(alias)))
            self.assertEqual(envelope["data"]["undo_entries"], 0)
            self.assertNotIn(
                "secret",
                result.stdout + result.stderr + self.state_path(alias).read_text(),
            )

    @unittest.skipUnless(REAL_CLI, "cli-anything-dokploy is not installed")
    def test_actual_cli_staging_undo_and_dry_run_do_not_cross_instances(self):
        def local_command(alias, *args):
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
                    *args,
                ],
                check=False,
                capture_output=True,
                text=True,
                timeout=15,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            return json.loads(result.stdout)["data"]

        for alias, instance in self.instances.items():
            with helper.bound_session(instance) as path:
                data = json.loads(path.read_text())
                data["target"] = {"compose_id": "shared-service-id"}
                path.write_text(json.dumps(data))
            local_command(alias, "env", "set", "INSTANCE=" + alias)
        for alias in self.instances:
            self.assertEqual(local_command(alias, "env", "staged"), {"INSTANCE": alias})
        path = self.state_path("home-server")
        self.assertEqual(len(json.loads(path.read_text())["undo"]), 1)
        local_command("home-server", "--dry-run", "env", "set", "INSTANCE=temporary")
        self.assertEqual(
            local_command("home-server", "env", "staged"), {"INSTANCE": "home-server"}
        )
        local_command("home-server", "session", "undo")
        self.assertEqual(local_command("home-server", "env", "staged"), {})
        self.assertEqual(
            local_command("quantforge", "env", "staged"), {"INSTANCE": "quantforge"}
        )
        self.assertEqual(
            local_command("quantmind", "env", "staged"), {"INSTANCE": "quantmind"}
        )


if __name__ == "__main__":
    unittest.main()
