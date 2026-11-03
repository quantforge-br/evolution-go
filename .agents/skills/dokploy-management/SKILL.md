---
name: dokploy-management
description: |
  Manage one or multiple live Dokploy PaaS instances by named aliases with the `dokploy-cli` command: projects, applications, compose services, databases (postgres, mysql, mariadb, mongo, redis, libsql), deploy/redeploy/start/stop/reload with wait, deployment and container logs, Docker containers/volumes/files on any node, domains, servers, backups, registries, SSH keys, environment variables, and any of the server's ~600 REST endpoints through `api call`.
  Use when the user wants to deploy or administer Dokploy resources, read deploy or runtime logs, browse container or volume files, automate a Dokploy server, select an instance by alias, compare servers, or access its host over SSH.
  Keywords: dokploy, paas, deploy, docker, traefik, application, compose, postgres, mysql, mariadb, mongo, redis, libsql, database, server, backup, domain, dns, certificate, registry, ssh-key, logs, deployment, container, volume, api.
compatibility: |
  Requires the `dokploy-cli` command on PATH (source: ~/dev/dokploy-cli, install with
  `uv tool install ~/dev/dokploy-cli`). Requires a reachable Dokploy server and an API key.
  Named-instance helper requires Python 3.11+ on Unix; SSH uses existing Host entries in ~/.ssh/config.
allowed-tools: Bash(dokploy-cli:*), Bash(python3:*), Bash(which:*)
---

# Dokploy management

`dokploy-cli` is a stateless CLI over a **live** Dokploy server. Backend operations
call `<url>/api/<router>.<procedure>` with the `x-api-key` header; help, version,
registry inspection and `--dry-run` are offline. Nothing is stored locally except
an optional endpoint-spec cache; there are no sessions, targets or local undo.
For ownership-checked installation/update instructions, read
[references/multi-instance.md](references/multi-instance.md#cli-installation-and-updates).

## Choose an instance before operating

Run every command through the bundled helper, which injects the alias's URL and
key into the CLI process only. Resolve SKILL_DIR from the location of this
SKILL.md, not the caller's working directory. Setup, credentials and SSH:
[references/multi-instance.md](references/multi-instance.md); starter config:
[assets/instances.example.toml](assets/instances.example.toml).

~~~bash
H="$SKILL_DIR/scripts/dokploy_instance.py"
python3 "$H" list --json                         # aliases, URLs, ssh hosts; reads no keys
python3 "$H" run diogosrodrigues -- --json doctor    # health, auth, server version, spec drift
python3 "$H" run quantforge -- --json service list
python3 "$H" ssh quantmind -- docker ps
~~~

In all examples below, `dokploy-cli ARGS` means `python3 "$H" run ALIAS -- ARGS`. Root
options (`--json`, `--timeout S`, `--show-secrets`) go right after `--`; `--url`
is refused.

Rules:
- Resolve the alias from the local registry, never from hardcoded URLs or keys.
- Keep a clearly selected instance in context but pass its alias on every call.
  If none is established or the request is ambiguous, ask; never guess production.
- For comparisons, query each alias separately and label results by instance.
  No cross-instance mutations without explicit scope.
- Use the API for Dokploy operations, SSH only for host investigation. The SSH host
  is the Dokploy host, not necessarily a service's remote Docker node.
- Before destructive operations, name the alias, resource and effect, and obtain
  authorization. SSH has no `--yes` protection.
- Unknown aliases and missing keys fail closed; never retry against another instance.

## Output contract

With `--json`, stdout is exactly one document: `{"ok": true, "data": ...}` or
`{"ok": false, "error": {"code", "message", "status"?, "detail"?}}`. The helper adds
only an `[instance ALIAS] URL` banner on stderr.

| Exit | Meaning | error.code |
|---|---|---|
| 0 | success | |
| 1 | API, network, deployment or output failure | `not_found`, `bad_request`, `unauthorized`, `forbidden`, `unreachable`, `deployment_failed`, `timeout`, `doctor_failed`, `serialization_error` |
| 2 | usage/validation or missing `--yes`; also helper config errors | `usage_error`, `bad_input`, `confirmation_required`, `unsupported` |
| 3 | URL/key not configured | `not_configured` |

Password/token/secret/apiKey/privateKey fields are masked as `"***"`; add
`--show-secrets` only when the user needs the value. GET retries twice on 5xx or
network errors; POST is never retried.
`serialization_error` returns a safe error without the invalid payload or a
traceback. It does not prove a write failed: inspect the selected instance's
state before retrying a mutation.

## Inspect

```bash
dokploy-cli --json project list                       # projectId, name, environments, service count
dokploy-cli --json project show PROJECT_ID            # environments with every service and its ids
dokploy-cli --json service list [--project NAME|ID] [--type compose] [--name TEXT]
dokploy-cli --json service show TYPE ID               # full record (<type>.one)
dokploy-cli --json service logs TYPE ID --tail 100 --since 30m [--search TEXT]
dokploy-cli --json service containers COMPOSE_ID      # compose logs need --container NAME|ID
dokploy-cli --json service logs compose ID --container web --tail 50
dokploy-cli --json deployment list TYPE ID            # application|compose, newest first
dokploy-cli --json deployment logs DEPLOYMENT_ID --tail 200
```

`TYPE` is `application, compose, postgres, mysql, mariadb, mongo, redis, libsql`.
`service list` rows carry `id`, `name`, `appName`, `type`, `projectName`,
`environmentName`, `status`, `lastDeployAt`; resolve names to ids there and use ids
in every later command. `--since` is `all` or `N[smhd]`.

## Lifecycle (requires `--yes`; preview with `--dry-run`)

```bash
dokploy-cli --json service deploy compose ID --yes --wait [--title "msg"] [--wait-timeout 900]
dokploy-cli --json service redeploy application ID --yes --wait
dokploy-cli --json service start|stop TYPE ID --yes
dokploy-cli --json service reload TYPE ID --yes        # application and databases, not compose
dokploy-cli --json deployment wait TYPE ID DEPLOYMENT_ID [--wait-timeout 900]
```

`--wait` (deploy/redeploy of application or compose) returns `data.deployment` and
exits 1 with `deployment_failed` when it ends `error`/`cancelled`; then read
`deployment logs <data.deployment.deploymentId>`. Unsupported combinations
(e.g. `redeploy redis`) fail with `unsupported` before any request.

## Any endpoint: `api`

```bash
dokploy-cli --json api list --domain docker           # ops, method, required params
dokploy-cli --json api list --search olume
dokploy-cli --json api describe compose.create        # name, type, in (query|body), required, enum
dokploy-cli --json api call OP -p key=value ... [--body JSON|@file] [--file field=PATH] [--yes|--dry-run]
```

`-p` values are typed from the registry (booleans `true/false`, numbers, JSON for
objects/arrays) and checked for unknown names, enums and required fields before
sending. Every non-GET call needs `--yes`. If `doctor` shows
`registry.matchesServer: false`, run `dokploy-cli spec refresh` once for that
instance (cached per URL) so `api` sees that server's exact endpoints.

## Workflows

### Deploy a raw compose stack in a new project

```bash
dokploy-cli --json api call project.create -p name=my-project --yes
# data.environment.environmentId is the default "production" environment
dokploy-cli --json api call compose.create -p name=my-web -p environmentId=ENV_ID \
  -p sourceType=raw -p composeFile="$(cat docker-compose.yml)" --yes
dokploy-cli --json api call compose.update -p composeId=COMPOSE_ID -p sourceType=raw \
  -p composeFile="$(cat docker-compose.yml)" --yes   # create may ignore the file; set it explicitly
dokploy-cli --json service deploy compose COMPOSE_ID --yes --wait
dokploy-cli --json service containers COMPOSE_ID
```

### Databases

```bash
dokploy-cli --json api describe postgres.create       # name, databaseName, databaseUser, databasePassword, environmentId required
dokploy-cli --json api call postgres.create -p name=my-db -p databaseName=app -p databaseUser=app \
  -p databasePassword="$PW" -p environmentId=ENV_ID --yes
dokploy-cli --json service deploy postgres DB_ID --yes
dokploy-cli --json service logs postgres DB_ID --tail 100
dokploy-cli --json api call postgres.changePassword -p postgresId=DB_ID -p password="$NEW" --yes
```

### Environment variables

`env` is one `KEY=value` newline-separated string and every save **replaces** it.
Read the current value first (`--show-secrets service show TYPE ID`, field `env`),
edit it, then write it back:

```bash
dokploy-cli --json api call compose.update -p composeId=ID --body @env.json --yes            # {"env": "..."}
dokploy-cli --json api call postgres.saveEnvironment -p postgresId=ID --body @env.json --yes
dokploy-cli --json api describe application.saveEnvironment   # also needs buildArgs, buildSecrets, createEnvFile
```

Then redeploy/reload so containers pick up the change.

### Domains

```bash
dokploy-cli --json api call domain.byComposeId -p composeId=ID
dokploy-cli --json api call domain.create -p host=app.example.com -p composeId=ID \
  -p serviceName=web -p port=3000 -p https=true -p certificateType=letsencrypt -p domainType=compose --yes
```

### Docker, files and volumes on a node

```bash
dokploy-cli --json api call docker.getContainers [-p serverId=SERVER_ID]
dokploy-cli --json api call docker.listContainerFiles -p containerId=CID -p path=/etc
dokploy-cli --json api call docker.readContainerFile -p containerId=CID -p path=/etc/hostname
dokploy-cli --json api call dockerVolume.getVolumes
dokploy-cli --json api call dockerVolume.listVolumeFiles -p volumeName=VOL -p path=/
dokploy-cli --json api call dockerVolume.readVolumeFile -p volumeName=VOL -p path=/status.json
dokploy-cli --json api call docker.uploadFileToContainer -p containerId=CID \
  -p destinationPath=/etc/app/local.conf --file file=./local.conf --yes
```

`serverId` aims a call at a registered remote Docker node of the same instance;
without it the Dokploy host is used. An alias selects the installation; `serverId`
never switches installations.

### Servers, backups, registries

```bash
dokploy-cli --json api call server.all
dokploy-cli --json api call destination.all
dokploy-cli --json api call registry.all
dokploy-cli --json api call sshKey.all
dokploy-cli --json api call deployment.queueList
```

## Safety rules

- All lifecycle commands and every non-GET `api call` need `--yes`; use `--dry-run`
  to show the exact request first.
- Server-side changes are immediate and not undoable.
- `project.remove` deletes only database rows: containers, networks and
  `/etc/dokploy/compose/<appName>` stay on the host. Remove services first
  (`compose.delete` with `deleteVolumes`, `application.delete`, `<db>.remove`).
- Host cleanup endpoints (`settings.cleanAll`, `cleanUnusedImages`,
  `cleanStoppedContainers`, `cleanDockerPrune`, `cleanUnusedVolumes`,
  `reloadServer`, `updateServer`) never run without explicit operator authorization.
- Never echo API keys; the helper passes them only in the child environment.

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| `Unknown operation 'x'. Did you mean ...` | Not in the registry; `api list --search` or `spec refresh` for a newer server |
| `Missing required parameter(s)` / `Unknown parameter` | `api describe OP` lists names, types and required fields |
| `confirmation_required` | Add `--yes` after confirming scope |
| `compose has no 'reload' endpoint` | Use `service redeploy` or `stop` + `start` |
| `choose --container from: ...` | Compose logs need `--container` (see `service containers`) |
| `not_found ... Server not found` | `serverId` is not registered on this instance |
| deploy ends `deployment_failed` | `deployment logs <id> --tail 200`; `Github Provider not found` means the compose lacks `sourceType=raw`; `all predefined address pools have been exhausted` is a Docker network shortage on the host |
| `doctor_failed` with `unauthorized` | Wrong or revoked key for that alias |
| `serialization_error` | Invalid command output; inspect server state before retrying writes, then report a CLI bug without including secrets |

## Verification

Helper tests (synthetic credentials, no server):
`python3 -m unittest discover -s "$SKILL_DIR/tests" -v`. The CLI's own unit and
live tests live in `~/dev/dokploy-cli/tests/`.
