# Named Dokploy instances

## Local setup

The helper requires Python 3.11+ (standard library only), a Unix host, and the
existing CLI on PATH. It does not add a native --instance flag to the CLI.
Resolve SKILL_DIR to the directory containing this skill's SKILL.md, not the
current working directory. For example, in this repository:

~~~bash
SKILL_DIR="$(pwd)/.agents/skills/dokploy-management"
python3 "$SKILL_DIR/scripts/dokploy_instance.py" --help
~~~

Store personal configuration outside the repository, by default at
~/.config/cli-anything-dokploy/instances.toml. XDG_CONFIG_HOME overrides
~/.config. An explicit --config PATH before the subcommand overrides this path.
The helper never chooses a global active instance.

~~~bash
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/cli-anything-dokploy"
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
python3 - "$CONFIG_DIR/secrets/home-server.key" <<'PY'
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

## SSH setup

Keep SSH addresses, ports, users, identity files and bastions in ~/.ssh/config:

~~~sshconfig
Host home-server
    HostName 203.0.113.10
    User deploy
    IdentityFile ~/.ssh/home-server
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
python3 "$SKILL_DIR/scripts/dokploy_instance.py" run home-server -- --json services list
python3 "$SKILL_DIR/scripts/dokploy_instance.py" run quantforge -- target use compose backend
python3 "$SKILL_DIR/scripts/dokploy_instance.py" run quantforge -- --json deployment list
python3 "$SKILL_DIR/scripts/dokploy_instance.py" ssh quantmind -- docker ps
~~~

For all workflows in SKILL.md, replace the leading cli-anything-dokploy with
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
Interactive SSH and the CLI REPL are intentionally unsupported: commands must
have an explicit operation so a persistent interpreter cannot change instances.

## Session isolation

Each alias has its own directory beneath
~/.local/state/cli-anything-dokploy/instances/<alias>/, or the corresponding
XDG_STATE_HOME directory. It contains session.json, a per-instance lock, and any
preview artifacts. Targets, staged env/compose, history and undo remain separate.
New session directories use mode 700 and files use 600. The API key exists only
in the CLI child process environment, never in the session or argv. Configured
API-key environment variables are removed from other child environments,
including SSH. The calling shell's environment is not modified.

The helper binds the session URL once, rejects --url/--session/--project
(including --option=value), session connect/disconnect, and REPL entry. A URL
change in configuration fails closed when an old session exists. For a different
server, prefer a new alias; archive the old alias's entire state directory only
after explicitly deciding to discard its target, staged changes and previews.
Changing the API key without changing the URL does not require resetting state.
An alias still refers to the same server if the server behind a URL is replaced;
archive the session explicitly after such a migration.

Operations on the same alias are serialized by a lock covering the entire child
process, preventing concurrent read/modify/write session loss. Different aliases
can run concurrently. Conversations using the same alias still share that alias's
target; reselect the requested service immediately before target-dependent work.
For multi-step mutations, prefer explicit IDs rather than relying on a target
that another conversation may change between commands.

## Troubleshooting

| Symptom | Action |
|---|---|
| Config not found | Create local instances.toml; do not silently use the legacy session |
| Unknown alias | List the configured aliases and ask which one the operator intended |
| Key file permissions refused | Set the file to 600 and its parent directory to 700; verify ownership |
| Missing credential | Populate the referenced file or environment variable, never print the key |
| Session URL differs | Check for an accidental URL edit; use a new alias or explicitly archive old state |
| SSH host key failure | Ask the operator to verify host identity; do not bypass StrictHostKeyChecking |
| SSH authentication failure | Check the configured user, identity and SSH agent; API access is independent |

## Local verification

Run helper unit/integration tests without a server or real credentials:

~~~bash
python3 -m unittest discover -s "$SKILL_DIR/tests" -v
~~~

Tests exercise three aliases, process environment and argument isolation,
per-instance sessions/previews, SSH quoting and host-key flags, fail-closed
routing, credential permissions, exit codes, and invocation of the actual CLI
against local state only when that CLI is installed.
