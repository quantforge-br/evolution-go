#!/usr/bin/env python3
"""Route Dokploy CLI and SSH commands through explicitly named local instances."""

import argparse
import json
import os
import re
import shlex
import shutil
import stat
import subprocess
import sys
import tomllib
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import urlsplit


class ConfigError(Exception):
    """A local configuration or routing error safe to show to the operator."""


@dataclass(frozen=True)
class Instance:
    name: str
    url: str
    ssh_host: str | None
    api_key_file: Path | None
    api_key_env: str | None


CLI = "dokploy-cli"


def default_config_path() -> Path:
    root = Path(os.environ.get("XDG_CONFIG_HOME", "~/.config")).expanduser()
    return root / "dokploy-cli" / "instances.toml"


def local_path(value: str, parent: Path) -> Path:
    path = Path(value).expanduser()
    return path if path.is_absolute() else parent / path


def normalize_url(value: str) -> str:
    if not isinstance(value, str) or not value or any(c.isspace() for c in value):
        raise ConfigError("Each instance needs an HTTP(S) base URL without whitespace.")
    try:
        parsed = urlsplit(value)
        port = parsed.port
        valid = (
            parsed.scheme in {"http", "https"}
            and parsed.hostname
            and parsed.username is None
            and parsed.password is None
            and not parsed.query
            and not parsed.fragment
            and (port is None or port > 0)
        )
    except ValueError:
        valid = False
    if not valid:
        raise ConfigError(
            "Invalid base URL; credentials, query strings and fragments are not allowed."
        )
    return value.rstrip("/")


def load_instances(path: Path) -> dict[str, Instance]:
    try:
        with path.open("rb") as handle:
            data = tomllib.load(handle)
    except FileNotFoundError:
        raise ConfigError(
            f"Instance config not found: {path}. See references/multi-instance.md."
        ) from None
    except (OSError, UnicodeError, tomllib.TOMLDecodeError):
        raise ConfigError(f"Cannot read instance config as TOML: {path}.") from None
    entries = data.get("instances")
    if set(data) != {"instances"} or not isinstance(entries, dict) or not entries:
        raise ConfigError("Config must contain one or more [instances.<alias>] tables.")
    instances = {}
    for name, entry in entries.items():
        if not re.fullmatch(r"[a-z][a-z0-9_-]{0,62}", name):
            raise ConfigError("Instance aliases must match [a-z][a-z0-9_-]{0,62}.")
        if not isinstance(entry, dict) or set(entry) - {
            "url",
            "ssh_host",
            "api_key_file",
            "api_key_env",
        }:
            raise ConfigError(
                f"Unsupported fields for instance '{name}'; store only credential references."
            )
        url = normalize_url(entry.get("url"))
        ssh_host = entry.get("ssh_host")
        if ssh_host is not None and (
            not isinstance(ssh_host, str)
            or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", ssh_host)
        ):
            raise ConfigError(
                f"Instance '{name}' needs a plain SSH Host alias from ~/.ssh/config."
            )
        key_file, key_env = entry.get("api_key_file"), entry.get("api_key_env")
        if (key_file is None) == (key_env is None):
            raise ConfigError(
                f"Instance '{name}' needs exactly one of api_key_file or api_key_env."
            )
        if key_file is not None and (
            not isinstance(key_file, str) or not key_file.strip()
        ):
            raise ConfigError(f"Invalid api_key_file reference for instance '{name}'.")
        if key_env is not None and (
            not isinstance(key_env, str)
            or not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", key_env)
            or key_env == "DOKPLOY_URL"
        ):
            raise ConfigError(f"Invalid api_key_env reference for instance '{name}'.")
        instances[name] = Instance(
            name,
            url,
            ssh_host,
            local_path(key_file, path.parent) if key_file is not None else None,
            key_env,
        )
    return instances


def read_api_key(instance: Instance) -> str:
    if instance.api_key_env is not None:
        value = os.environ.get(instance.api_key_env, "")
    else:
        try:
            with instance.api_key_file.open(encoding="utf-8") as handle:
                info = os.fstat(handle.fileno())
                if (
                    not stat.S_ISREG(info.st_mode)
                    or info.st_uid != os.getuid()
                    or info.st_mode & 0o077
                ):
                    raise ConfigError(
                        "API key file must be owned by the current user with mode 600 (or 400)."
                    )
                value = handle.read()
        except (OSError, UnicodeError):
            raise ConfigError(
                f"Cannot read API key file for instance '{instance.name}'."
            ) from None
    value = value.strip()
    if not value or any(c.isspace() or ord(c) < 32 or ord(c) == 127 for c in value):
        raise ConfigError(
            f"Missing or invalid API key for instance '{instance.name}'; no fallback is used."
        )
    return value


def child_environment(instances: dict[str, Instance]) -> dict[str, str]:
    env = os.environ.copy()
    for name in {"DOKPLOY_API_KEY", "DOKPLOY_URL"} | {
        instance.api_key_env for instance in instances.values() if instance.api_key_env
    }:
        env.pop(name, None)
    return env


def validate_cli_args(args: list[str]) -> None:
    if not args:
        raise ConfigError("Provide a dokploy-cli command after '--'.")
    for arg in args:
        if arg.split("=", 1)[0] == "--url":
            raise ConfigError(
                "URL overrides are not allowed; choose an instance alias instead."
            )
    # Only root options can precede a command; do not interpret option values as commands.
    position = 0
    while position < len(args):
        arg = args[position]
        if arg in {"--json", "--show-secrets"}:
            position += 1
        elif arg == "--timeout":
            position += 2
        elif arg.startswith("--timeout="):
            position += 1
        elif arg.startswith("-"):
            if arg in {"--help", "--version"} and position == len(args) - 1:
                return
            raise ConfigError(
                "Unsupported root option; put command options after the command."
            )
        else:
            return
    raise ConfigError("Provide a dokploy-cli command after the root options.")


def executable(name: str) -> str:
    path = shutil.which(name)
    if path is None:
        raise ConfigError(f"Required executable not found on PATH: {name}.")
    return path


def run_cli(instance: Instance, instances: dict[str, Instance], args: list[str]) -> int:
    validate_cli_args(args)
    program = executable(CLI)
    key = read_api_key(instance)
    env = child_environment(instances)
    env.update(DOKPLOY_URL=instance.url, DOKPLOY_API_KEY=key)
    print(f"[instance {instance.name}] {instance.url}", file=sys.stderr, flush=True)
    result = subprocess.run([program, *args], check=False, env=env, umask=0o077)
    return result.returncode


def run_ssh(instance: Instance, instances: dict[str, Instance], args: list[str]) -> int:
    if not instance.ssh_host:
        raise ConfigError(f"No ssh_host configured for instance '{instance.name}'.")
    if not args:
        raise ConfigError(
            "Provide an explicit remote command after '--'; interactive SSH is not supported."
        )
    program = executable("ssh")
    print(
        f"[instance {instance.name}] SSH {instance.ssh_host}",
        file=sys.stderr,
        flush=True,
    )
    result = subprocess.run(
        [
            program,
            "-o",
            "BatchMode=yes",
            "-o",
            "StrictHostKeyChecking=yes",
            "--",
            instance.ssh_host,
            shlex.join(args),
        ],
        check=False,
        env=child_environment(instances),
        umask=0o077,
    )
    return result.returncode


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--config",
        type=Path,
        default=default_config_path(),
        help="Local instances.toml path",
    )
    commands = parser.add_subparsers(dest="command", required=True)
    listing = commands.add_parser(
        "list", help="List aliases without reading API keys or connecting"
    )
    listing.add_argument(
        "--json", action="store_true", help="Machine-readable instance metadata"
    )
    for name in ("run", "ssh"):
        command = commands.add_parser(
            name, help=f"Run an explicit {name} command on one instance"
        )
        command.add_argument("alias")
        command.add_argument(
            "args", nargs=argparse.REMAINDER, help="Command arguments following '--'"
        )
    return parser


def main(argv: list[str] | None = None) -> int:
    options = build_parser().parse_args(argv)
    try:
        instances = load_instances(options.config.expanduser().absolute())
        if options.command == "list":
            rows = [
                {
                    "alias": item.name,
                    "url": item.url,
                    "ssh_host": item.ssh_host,
                    "credential_source": "file" if item.api_key_file else "environment",
                }
                for item in sorted(instances.values(), key=lambda item: item.name)
            ]
            if options.json:
                print(json.dumps({"success": True, "data": rows}, indent=2))
            else:
                for row in rows:
                    print(
                        f"{row['alias']}\t{row['url']}\tssh={row['ssh_host'] or '-'}\tkey={row['credential_source']}"
                    )
            return 0
        if options.alias not in instances:
            raise ConfigError(
                "Unknown instance alias; use 'list' to see the configured aliases. No fallback is used."
            )
        args = options.args[1:] if options.args[:1] == ["--"] else options.args
        action = run_cli if options.command == "run" else run_ssh
        result = action(instances[options.alias], instances, args)
        return result if result >= 0 else 128 - result
    except (ConfigError, OSError) as error:
        message = (
            str(error)
            if isinstance(error, ConfigError)
            else "Local execution failed; check paths and permissions."
        )
        print(json.dumps({"success": False, "error": message}), file=sys.stderr)
        return 2
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
