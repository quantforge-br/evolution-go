---
name: dokploy-management
description: |
  Manage one or multiple live Dokploy PaaS instances by named aliases from the command line with `cli-anything-dokploy`: projects, applications, compose services, databases (postgres, mysql, mariadb, mongo, redis, libsql), deployments and their logs, Docker containers/images/volumes/networks/disk on any node, domains and DNS providers, remote servers, volume backups, registries, SSH keys, environment variables, and any of the server's REST endpoints through `api call`.
  Use when the user wants to deploy or administer Dokploy resources, point a session at a service, read deploy logs, browse container or volume files, automate a Dokploy server, select an instance by alias, compare servers, or access its host over SSH.
  Keywords: dokploy, paas, deploy, docker, traefik, application, compose, postgres, mysql, mariadb, mongo, redis, libsql, database, server, backup, domain, dns, certificate, registry, ssh-key, logs, deployment, container, volume, session, target, api.
compatibility: |
  Requires the `cli-anything-dokploy` command on PATH (pip: `pip install cli-anything-dokploy`,
  source: `pip install -e .` inside the project's `agent-harness/` directory).
  Requires a reachable Dokploy server and an API key. Named-instance helper requires Python 3.11+
  on Unix; SSH uses existing Host entries in ~/.ssh/config.
allowed-tools: Bash(cli-anything-dokploy:*), Bash(python3:*), Bash(which:*), Bash(export:*)
---

# Dokploy management

`cli-anything-dokploy` is a stateful CLI over a **live** Dokploy server. It never
emulates Dokploy: every command is a real call to `<base>/api/<domain>.<action>`
with the `x-api-key` header. The 600+ endpoint catalogue comes from the server's
own OpenAPI document, so a newer server exposes its newer endpoints without a
CLI upgrade.

## Choose an instance before operating

For named instances, use the bundled helper rather than changing global
DOKPLOY_URL/DOKPLOY_API_KEY variables. Resolve SKILL_DIR from the location of
this SKILL.md, not the caller's working directory. Read
[references/multi-instance.md](references/multi-instance.md) for local setup,
credential references, SSH, isolation, and troubleshooting. The starter config
is [assets/instances.example.toml](assets/instances.example.toml); real endpoints
and keys belong outside the repository.

~~~bash
python3 "$SKILL_DIR/scripts/dokploy_instance.py" list --json
python3 "$SKILL_DIR/scripts/dokploy_instance.py" run home-server -- --json services list
python3 "$SKILL_DIR/scripts/dokploy_instance.py" run quantforge -- target use compose backend
python3 "$SKILL_DIR/scripts/dokploy_instance.py" run quantforge -- compose status
python3 "$SKILL_DIR/scripts/dokploy_instance.py" ssh quantmind -- docker ps
~~~

Apply these interaction rules:
- Resolve the user's alias from the local registry, never from hardcoded URLs or
  keys in this skill. List metadata without reading or printing key contents.
- Keep a clearly selected instance in conversational context, but pass its alias
  explicitly on every invocation. If no instance is established or the request
  is ambiguous, ask which one; never guess a production destination.
- For comparisons, query each requested alias separately and label every result
  by instance. Do not perform cross-instance mutations without explicit scope.
- Use the API for Dokploy operations, SSH for host investigation when needed.
  The configured SSH host is not necessarily a service's remote Docker node.
- Before destructive operations, identify the alias, destination and resource,
  and obtain authorization for that scope. SSH has no CLI --yes protection.
- Unknown aliases, missing keys and mismatched session URLs fail closed; never
  retry against a default or another instance.
- Do not use a global active-instance file, REPL, or connection/session overrides.
  Sessions and previews are isolated per alias. Concurrent commands for one alias
  are serialized, but its target is shared: reselect the requested service or
  prefer explicit IDs before target-dependent mutations.

In every command example below, named-instance operation means replacing
cli-anything-dokploy with
python3 "$SKILL_DIR/scripts/dokploy_instance.py" run ALIAS --.
Do not use session connect/disconnect with the helper: configure its local TOML
instead. The helper keeps stdout/JSON unchanged and labels the destination on
stderr. API keys are injected only into the CLI child environment.

### Legacy single-instance setup

Use direct CLI invocation only when the operator intentionally uses the legacy
single-instance environment, not as a fallback for a named-instance error.

~~~bash
which cli-anything-dokploy || echo "install it first"
export DOKPLOY_URL="https://dokploy.example.com"
# Supply DOKPLOY_API_KEY securely in the environment, never echo it.
cli-anything-dokploy session connect --url "$DOKPLOY_URL"
~~~

The CLI reads its API key from DOKPLOY_API_KEY and never saves it in the session.
For named instances the helper supplies that variable from a secure file or a
referenced environment variable, separately for each command.

## Session and target (the stateful part)

Each alias uses its own session under
~/.local/state/cli-anything-dokploy/instances/<alias>/session.json (respecting
XDG_STATE_HOME). The following defaults describe direct legacy CLI usage.
Everything is driven by a session file (default
`~/.local/state/cli-anything-dokploy/session.json`, overridable with `--session`
or `CLI_ANYTHING_DOKPLOY_SESSION`). It stores the URL, the current target, staged
env vars and the undo history. `--dry-run` runs a command without saving.

```bash
cli-anything-dokploy session connect --url "$DOKPLOY_URL"
cli-anything-dokploy session status
cli-anything-dokploy services list                 # every deployed service on the server
cli-anything-dokploy target use compose dsr-aur    # project/environment/service ids + node
cli-anything-dokploy target show
cli-anything-dokploy compose status                # no id needed: the target supplies it
cli-anything-dokploy target clear
```

`target use <kind> <name>` accepts `application`, `compose`, or any database type
(`postgres`, `mysql`, `mariadb`, `mongo`, `redis`, `libsql`) and records the
service's Docker node too, so later docker/database commands hit the right node
without `--server-id`. Pointing at a main-host service clears a previously stored
node. A name that matches several services is refused: pass the full name.

## Command surface

`--json` is a group-level flag: put it before the command. Errors go to stderr as
`{"success": false, "error": ...}` and exit non-zero, so shell pipelines stay clean.

| Group | What it does |
|---|---|
| `session connect \| status \| history \| save \| disconnect \| undo` | Connection, history, one-step undo |
| `target use KIND NAME \| show \| clear` | Point the session at a service |
| `project list \| stats \| info --project-id ID \| create NAME \| delete --project-id ID --yes` | Projects |
| `services list` | Every deployed service, resolved from the server's own aggregate |
| `overview summary \| domains` | Dashboard aggregates |
| `application \| compose \| db` — `info \| list \| deploy \| redeploy \| start \| stop \| status \| logs` | Lifecycle, id from `--id` or the target |
| `compose stage --file F \| push-file --yes` | Stage a compose file locally, then push it |
| `db --type postgres create NAME \| change-password P --yes` | Databases; `--type` picks the engine |
| `deployment list \| queue \| logs ID \| watch ID \| deploy-watch --yes \| kill ID --yes` | Deployment records, queue, logs, live watch |
| `docker containers \| images \| volumes \| networks \| disk \| build-cache` | Docker on the node |
| `docker container ACTION ID --yes \| container-config ID \| events \| health` | Container operations and node health |
| `docker container-files ID [--path /] \| read-container-file ID PATH` | Browse and read files inside a container |
| `docker volume-files VOL [--path /] \| read-volume-file VOL PATH \| upload-file ID LOCAL DEST` | Volume contents, container uploads |
| `domain list-app \| create HOST --yes \| validate HOST \| dns-providers \| dns-zones P \| dns-records P Z` | Domains, DNS, certificates |
| `server list \| backups [--id ID] [--type T] \| destinations \| registries \| ssh-keys` | Servers, per-service volume backups, destinations, registries, keys |
| `env get \| set KEY[=VALUE] \| unset KEY \| staged \| push --yes` | Stage env vars locally, then push |
| `preview capture RECIPE \| latest \| diff \| live start\|push\|status\|stop` | Inspection bundles of real server state |
| `api domains \| actions DOMAIN \| params DOMAIN ACTION \| call DOMAIN ACTION [--param K=V]… [--yes]` | Any endpoint the server has |

`api call` is the escape hatch: every endpoint in the server's registry is
reachable, with `--param` per field (file fields take a local path). Non-GET calls
require `--yes`.

## Workflows

### Deploy a project from a compose file

```bash
ENV_ID=$(cli-anything-dokploy --json project create my-project \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["data"]["environment_id"])')
cli-anything-dokploy --json compose create my-web --file docker-compose.yml --environment-id "$ENV_ID"
cli-anything-dokploy target use compose my-web
cli-anything-dokploy --json deployment deploy-watch --yes --timeout 300
cli-anything-dokploy compose status
cli-anything-dokploy docker containers
```

`compose create --file` creates a `raw` compose service (file stored on the
server). Without `--file` the service is created as a git source. `deploy-watch`
deploys once, waits for the new deployment record and returns its final status;
`--json` gives `deploymentId`, `status` and `errorMessage`.

### Databases

```bash
cli-anything-dokploy --json db --type postgres create my-db --environment-id "$ENV_ID"
# prints the generated user/password; keep them
cli-anything-dokploy target use postgres my-db
cli-anything-dokploy db --type postgres deploy
cli-anything-dokploy db --type postgres logs --tail 100
cli-anything-dokploy db --type postgres change-password 'NEW-PASSWORD' --yes
cli-anything-dokploy --json api call postgres remove --param postgresId=<ID> --yes
```

Database name and user default to the service name; the password is generated when
omitted and returned in the create response.

### Logs and deployments

```bash
cli-anything-dokploy deployment list
cli-anything-dokploy deployment logs <deploymentId> --tail 200 --download deploy.log
cli-anything-dokploy deployment watch <deploymentId> --timeout 180
cli-anything-dokploy deployment queue
```

In `--json` mode an artifact note (the `--download` path) goes to stderr, so stdout
stays pure JSON.

### Environment variables

```bash
cli-anything-dokploy env set API_TOKEN=xxxx        # staged in the session, auto-saved
cli-anything-dokploy env staged
cli-anything-dokploy env get                       # server blob + staged + merged
cli-anything-dokploy env push --yes                # overwrites the server env (confirmed)
cli-anything-dokploy env unset API_TOKEN
```

`env push` writes the merged blob; keys already on the server are replaced, not
duplicated.

### Files inside containers and volumes

```bash
cli-anything-dokploy docker container-files <containerId> --path /etc
cli-anything-dokploy docker read-container-file <containerId> /etc/hostname
cli-anything-dokploy docker volumes
cli-anything-dokploy docker volume-files <volume> --path /
cli-anything-dokploy docker read-volume-file <volume> /status.json
cli-anything-dokploy docker upload-file <containerId> ./local.conf /etc/app/local.conf
```

Contents arrive base64-encoded from Dokploy and are decoded for you; a truncated
read is reported on stderr.

### Inspection bundles

```bash
cli-anything-dokploy preview capture deployment     # real record + real log tail
cli-anything-dokploy preview latest services
cli-anything-dokploy preview diff services <A> <B>
cli-anything-dokploy --json preview live start services
cli-anything-dokploy --json preview live status
```

Bundles follow the `preview-bundle/v1` protocol with `manifest.json`,
`summary.json` and `artifacts/`, consumed by `cli-hub previews inspect|html|watch`.

### Any other endpoint

```bash
cli-anything-dokploy api domains                       # 57 domains, live counts
cli-anything-dokploy api actions notification
cli-anything-dokploy api params notification createDiscord
cli-anything-dokploy api call notification createDiscord --param name=alerts \
  --param webhookUrl=https://... --yes
```

The registry is fetched from the connected server and cached per URL under
`~/.cache/cli-anything-dokploy/registries/`; the bundled copy is only a fallback
when the server cannot answer. `api actions` is therefore the authoritative list
for *your* server version.

## Remote nodes

An instance alias chooses an independent Dokploy installation. --server-id does
not switch installations: it chooses a Docker node registered in that instance.

`--server-id` aims a command at a registered Docker node. Without it, the node
stored in the session target is used, and with no target the main Dokploy host is
addressed. `docker.getContainers?serverId=…` against an id the server does not know
fails with `404 … on server '<id>': Server not found`, which is how you can tell
routing happened.

## Safety rules

- Destructive actions require `--yes`: `project delete`, `domain create`,
  `docker container remove|kill|stop`, `docker prune-build-cache`,
  `deployment kill`, `env push`, `compose push-file`, `db change-password`,
  `application traefik --file`, and any non-GET `api call`.
- `project delete` always requires an explicit `--project-id` and refuses the
  session's own project, so a stray delete cannot take out the service you are
  pointed at.
- Server-side changes are not undoable. `session undo` only reverts local session
  state (target, staged env). Read `session history` to see what was called.
- Cleanup endpoints that prune host resources (`settings cleanAll`,
  `cleanUnusedImages`, `cleanStoppedContainers`, `cleanDockerPrune`,
  `reloadServer`, `updateServer`) are never run without explicit operator
  authorization.
- Never echo `DOKPLOY_API_KEY`. It is redacted from the session file, history and
  error details; keep it that way in your own output.

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| `unknown action '<x>' for domain '<y>'` | The endpoint does not exist on that server; `api actions <domain>` lists what it does have |
| `404 NOT_FOUND … Server not found` | The `serverId` is not registered on this instance; clear it or pass the right `--server-id` |
| `no application selected` / `no postgres selected` | Point the session: `target use <kind> <name>` |
| `no environment in the session` | The target has no environment: `target use` a service, or `project create` first (it stores the new environment) |
| `missing required parameter(s) for <domain>.<action>: …` | `api params <domain> <action>` shows the required fields |
| `deploy` ends with `status: error` | `deployment logs <id> --tail 200` shows the build error; a `Github Provider not found` means the service was created without `sourceType=raw`, and `all predefined address pools have been exhausted` is a Docker network shortage on the host |
| `refusing to … without --yes` | Add `--yes` (see Safety rules) |

## Reference: this skill in practice

Named-instance helper tests run locally with:
python3 -m unittest discover -s "$SKILL_DIR/tests" -v. They use synthetic
credentials, fake child processes, and local-only CLI operations. This is
separate from the single-instance live validation described below.

Validated against a live single-node instance (`v0.30.7`): services, target
resolution, deploy loop with `deploy-watch`, deployment logs and downloads, docker
inventories/disk/build cache/events/health, container and volume file reads,
database create → deploy → logs → change-password → start → stop → remove, env
staging and push, project create/delete guards, preview bundles, and `api call` on
raw endpoints. The same suite lives in the project at
`agent-harness/cli_anything/dokploy/tests/` (unit + live E2E, no skips).
