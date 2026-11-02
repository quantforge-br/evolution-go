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
