# Named Dokploy instances

## Local setup

The helper requires Python 3.11+ (standard library only), a Unix host, and
`dokploy-cli` on PATH. It does not add a native --instance flag to the CLI.
Resolve SKILL_DIR to the directory containing this skill's SKILL.md, not the
current working directory. For example, in this repository:

~~~bash
SKILL_DIR="$(pwd)/.agents/skills/dokploy-management"
python3 "$SKILL_DIR/scripts/dokploy_instance.py" --help
~~~

Store personal configuration outside the repository, by default at
~/.config/dokploy-cli/instances.toml. XDG_CONFIG_HOME overrides ~/.config.
An explicit --config PATH before the subcommand overrides this path.
The helper never chooses a global active instance.

~~~bash
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/dokploy-cli"
install -d -m 700 "$CONFIG_DIR" "$CONFIG_DIR/secrets"
# Only copy the example when instances.toml does not already exist.
test -e "$CONFIG_DIR/instances.toml" || \
  install -m 600 "$SKILL_DIR/assets/instances.example.toml" "$CONFIG_DIR/instances.toml"
~~~

Edit the example URLs and SSH aliases. Each [instances.<alias>] table accepts:

| Field | Meaning |
|---|---|
| url | HTTP(S) Dokploy base URL, without /api, credentials, query, or fragment |
| ssh_host | Optional Host alias from ~/.ssh/config, not an SSH command |
| api_key_file | File containing only the API key; ~ expands, relative paths resolve next to instances.toml |
| api_key_env | Environment variable holding the API key; use instead of api_key_file |

Aliases match [a-z][a-z0-9_-]{0,62}. There must be exactly one credential
reference per instance. Never put a literal key in TOML, SKILL.md, examples,
commands, or committed files. A secure file is the simple default. For example,
have the operator enter a key locally without placing it in shell history:

~~~bash
python3 - "$CONFIG_DIR/secrets/diogosrodrigues.key" <<'PY'
import getpass
import os
import sys
from pathlib import Path

path = Path(sys.argv[1])
key = getpass.getpass("Dokploy API key: ").strip()
if not key or any(character.isspace() for character in key):
    raise SystemExit("A non-empty API key without whitespace is required.")
fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, "w") as handle:
    handle.write(key + "\n")
PY
~~~

Repeat for the other aliases. Files must be regular, owned by the current user,
and accessible only to that user (600 or 400); directories should be 700.
These permissions are not encryption. For a secret manager, inject a dedicated
environment variable into the helper process and set api_key_env to its name.
An absent or empty credential never falls back to another instance's key.

## CLI installation and updates

The source is `~/dev/dokploy-cli`. Inspect command resolution and the uv receipt
before any installation; never replace a different tool just because its name
matches. A fresh installation, with no command conflict, is:

~~~bash
uv tool install "$HOME/dev/dokploy-cli"
~~~

For an explicitly requested update or recovery of this repository's tool:

~~~bash
GENERATOR="$HOME/.agents/skills/cli-generate/scripts/cli_generate.py"
python3 "$GENERATOR" check --repo "$HOME/dev/dokploy-cli"
python3 "$GENERATOR" install --reinstall --repo "$HOME/dev/dokploy-cli"
~~~

The helper refuses foreign uv receipts/entrypoints and never uses `--force`.
It verifies help, metadata-derived version, read-only `doctor`, and the packaged
usage-skill hash outside the source tree. The helper's `doctor` invokes the CLI
directly, so provide the explicitly selected instance's `DOKPLOY_URL` and
`DOKPLOY_API_KEY` in its process environment through the approved credential
source; never print them or substitute another instance. Do not use a mutating
command as an installation smoke test. After installing, verify the intended
alias with `python3 "$SKILL_DIR/scripts/dokploy_instance.py" run ALIAS -- --json doctor`.

`uv tool dir --bin` must be an absolute PATH entry. Python downloads are disabled;
set `UV_PYTHON` to an existing compatible interpreter if selection fails. Local
installation state belongs in ignored `.cli-generate-installation.json`, never
in versioned `.cli-generate.json`. These files describe the development install,
not instance credentials, sessions or targets. A blocked state needs inspection,
not an automatic retry or takeover. Installing the CLI never registers its
packaged usage skill globally and does not replace this management skill.

## SSH setup

Keep SSH addresses, ports, users, identity files and bastions in ~/.ssh/config:

~~~sshconfig
Host diogosrodrigues
    HostName 203.0.113.10
    User deploy
    IdentityFile ~/.ssh/diogosrodrigues
    IdentitiesOnly yes
~~~

The helper reuses these settings. BatchMode=yes and StrictHostKeyChecking=yes
prevent password prompts and unverified host acceptance. If the host key is
unknown or changed, stop and have the operator verify it through a trusted
channel. Do not disable host verification or add unverified ssh-keyscan output.
SSH targets the configured Dokploy host, not a service's remote Docker node.

## Commands

~~~bash
python3 "$SKILL_DIR/scripts/dokploy_instance.py" list
python3 "$SKILL_DIR/scripts/dokploy_instance.py" list --json
python3 "$SKILL_DIR/scripts/dokploy_instance.py" run diogosrodrigues -- --json service list
python3 "$SKILL_DIR/scripts/dokploy_instance.py" run quantforge -- --json deployment list compose COMPOSE_ID
python3 "$SKILL_DIR/scripts/dokploy_instance.py" ssh quantmind -- docker ps
~~~

For all workflows in SKILL.md, replace the leading dokploy-cli with
python3 "$SKILL_DIR/scripts/dokploy_instance.py" run ALIAS --. Put --json and
other CLI root options after this separator, before the CLI command.
The helper preserves CLI stdout and exit codes. An instance/destination banner
goes only to stderr; list --json has a success/data envelope. Local errors are
JSON on stderr with exit code 2. Listing requires neither credentials nor a
network connection, and SSH does not read the API key.

SSH arguments are shell-quoted individually before OpenSSH sends the remote
command, so metacharacters are literal arguments rather than implicit pipelines.
When a remote pipeline is actually needed, explicitly use sh -c and review the
whole remote script. SSH can bypass the API's --yes guards: apply the skill's
confirmation rules before any mutating remote command.
Interactive SSH is intentionally unsupported: commands must have an explicit
operation.

## Instance isolation

`dokploy-cli` is stateless, so nothing per alias is stored by the helper. Each
run starts the CLI with DOKPLOY_URL and DOKPLOY_API_KEY for that alias only; the
key never appears in argv. DOKPLOY_URL, DOKPLOY_API_KEY and every configured
API-key environment variable are removed from the child environment first, and
from SSH children entirely. The calling shell's environment is not modified.

The helper rejects `--url` (including `--url=value`) anywhere in the arguments and
any unknown root option before the command. The only state the CLI writes is the
optional `spec refresh` cache, keyed by URL under
`${XDG_CACHE_HOME:-~/.cache}/dokploy-cli/`, so aliases never share it. Different
aliases, and the same alias, can run concurrently.

## Troubleshooting

| Symptom | Action |
|---|---|
| Config not found | Create local instances.toml; never fall back to global DOKPLOY_URL/DOKPLOY_API_KEY |
| Unknown alias | List the configured aliases and ask which one the operator intended |
| Key file permissions refused | Set the file to 600 and its parent directory to 700; verify ownership |
| Missing credential | Populate the referenced file or environment variable, never print the key |
| SSH host key failure | Ask the operator to verify host identity; do not bypass StrictHostKeyChecking |
| SSH authentication failure | Check the configured user, identity and SSH agent; API access is independent |

## Local verification

Run helper unit/integration tests without a server or real credentials:

~~~bash
python3 -m unittest discover -s "$SKILL_DIR/tests" -v
~~~

Tests exercise three aliases, process environment and argument isolation, SSH
quoting and host-key flags, fail-closed routing, credential permissions, exit
codes, the XDG config default, and, when `dokploy-cli` is installed, a
`--dry-run` through the real CLI that sends no request.
