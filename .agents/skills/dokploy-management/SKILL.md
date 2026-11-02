---
name: dokploy-management
description: |
  Manage a live Dokploy PaaS instance from the command line with `cli-anything-dokploy`: projects, applications, compose services, databases (postgres, mysql, mariadb, mongo, redis, libsql), deployments and their logs, Docker containers/images/volumes/networks/disk on any node, domains and DNS providers, remote servers, volume backups, registries, SSH keys, environment variables, and any of the server's REST endpoints through `api call`.
  Use when the user wants to deploy or administer Dokploy resources, point a session at a service, read deploy logs, browse container or volume files, or automate a Dokploy server.
  Keywords: dokploy, paas, deploy, docker, traefik, application, compose, postgres, mysql, mariadb, mongo, redis, libsql, database, server, backup, domain, dns, certificate, registry, ssh-key, logs, deployment, container, volume, session, target, api.
compatibility: |
  Requires the `cli-anything-dokploy` command on PATH (pip: `pip install cli-anything-dokploy`,
  source: `pip install -e .` inside the project's `agent-harness/` directory).
  Requires a reachable Dokploy server and an API key.
allowed-tools: Bash(cli-anything-dokploy:*), Bash(which:*), Bash(export:*)
---

# Dokploy management

`cli-anything-dokploy` is a stateful CLI over a **live** Dokploy server. It never
emulates Dokploy: every command is a real call to `<base>/api/<domain>.<action>`
with the `x-api-key` header. The 600+ endpoint catalogue comes from the server's
own OpenAPI document, so a newer server exposes its newer endpoints without a
CLI upgrade.

## Before you start

```bash
which cli-anything-dokploy || echo "install it first"
export DOKPLOY_URL="https://dokploy.example.com"
export DOKPLOY_API_KEY="<key>"        # read from the environment only, never saved
```

The API key is only ever read from `DOKPLOY_API_KEY`; it is never written to the
session file. The URL may also be stored in the session with
`session connect --url`, and `--url` overrides it for a single run.

## Session and target (the stateful part)

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

Validated against a live single-node instance (`v0.30.7`): services, target
resolution, deploy loop with `deploy-watch`, deployment logs and downloads, docker
inventories/disk/build cache/events/health, container and volume file reads,
database create → deploy → logs → change-password → start → stop → remove, env
staging and push, project create/delete guards, preview bundles, and `api call` on
raw endpoints. The same suite lives in the project at
`agent-harness/cli_anything/dokploy/tests/` (unit + live E2E, no skips).
