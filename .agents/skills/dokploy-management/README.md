# dokploy-management skill: development notes

## Purpose

Operate one or more live Dokploy instances by alias through `dokploy-cli`
(source and tests: `~/dev/dokploy-cli`). The skill holds no API client of its own:
`scripts/dokploy_instance.py` only resolves an alias from a local TOML registry,
injects `DOKPLOY_URL`/`DOKPLOY_API_KEY` into the child process, and runs
`dokploy-cli` or `ssh`.

## Layout

- `SKILL.md`: agent-facing usage.
- `scripts/dokploy_instance.py`: `list`, `run ALIAS -- <dokploy-cli args>`, `ssh ALIAS -- <cmd>`.
- `references/multi-instance.md`: config, credentials, SSH, isolation.
- `assets/instances.example.toml`: starter config (placeholder URLs).
- `tests/test_instances.py`: `python3 -m unittest discover -s tests -v`.

## Copies

Keep `~/dev/dokploy/.agents/skills/dokploy-management` and
`~/dev/skills/dokploy-management` byte-identical.

## CLI compatibility and maintenance

Use the ownership-checked update/recovery flow in
`references/multi-instance.md#cli-installation-and-updates`. The CLI now derives
its version from distribution metadata and reports malformed output with exit 1
and `serialization_error`; the helper preserves that envelope and exit code.
Root options such as `--show-secrets` precede the CLI subcommand. Updates do not
change the alias registry or credentials and do not install usage skills globally.

## Changelog

### 2026-11-02: switched from `cli-anything-dokploy` to `dokploy-cli`

- The helper runs `dokploy-cli` (stateless) instead of `cli-anything-dokploy --session`.
  Removed per-alias session files, locks and the session URL binding; the old
  `~/.local/state/cli-anything-dokploy/` state was deleted.
- Config lives at `~/.config/dokploy-cli/instances.toml` (moved from
  `~/.config/cli-anything-dokploy/`; no fallback to the old path).
- Accepted root options: `--json`, `--timeout`, `--show-secrets`; `--url` is refused.
- SKILL.md rewritten for the `dokploy-cli` surface. Former convenience commands
  (target, env staging, docker file browsing, domains, previews) map to
  `service ...` or `api call <router.procedure>`; previews and local undo have no
  equivalent.
- Validated on 2026-11-02 against home-server, quantforge and quantmind (all v0.30.10):
  `doctor`, `service list`, `project list`, `docker.getContainers`,
  `dockerVolume.getVolumes`, `domain.create --dry-run`, `--url` rejection.

## Post-review compatibility verification

~~~text
$ PYTHONDONTWRITEBYTECODE=1 python3 -B -m unittest discover -s tests -v
Ran 14 tests
OK
~~~

These checks use synthetic credentials only. The real installed CLI is exercised
through each alias with a request-free dry run, including global --json and
--show-secrets options. No live writes or SSH commands are part of this update.
