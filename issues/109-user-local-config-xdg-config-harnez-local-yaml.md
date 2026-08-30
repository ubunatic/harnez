# 109 — User-local config: `~/.config/harnez/local.yaml`

**Status**: Closed — resolved in 984fc99
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [issue 051](051-multi-host-remote-monitoring-and-dashboard-navigation.md) (multi-host),
[issue 104](104-agy-quota-collector-requires-live-process-poll-coincidence.md),
`config.yaml` (shared committed config), `AGENTS.local.md` (ephemeral local overlay pattern)

## Problem

`config.yaml` is the canonical shared config — it is committed, managed by harnez,
and intentionally stable. It cannot carry machine-specific or user-specific values
(hostnames, local paths, personal defaults) because those would create per-machine
drift in the committed file.

There is currently no persistent store for machine-local static values. As a result:

- `--host um760` must be typed on every `harnez usage` invocation.
- The `Makefile` carries `HOST ?= um760` as a build-time workaround.
- Future machine-local state (hostname cache, discovered machine identity, etc.)
  has nowhere to live.

## Design Decision

Adopt **`~/.config/harnez/local.yaml`** (`$XDG_CONFIG_HOME/harnez/local.yaml`) as
the user-local config layer. Rationale (from design session 2026-08-30):

- We already work with `~/` paths (`~/.claude`, `~/.gemini`, etc.) — XDG is natural.
- The file starts nearly empty; almost no drift to worry about.
- It is outside the repo by definition — no gitignore ceremony required.
- It doubles as a safe home for future machine-local caching (hostname cache,
  discovered machine name, etc.) — not just a "hosts config".
- Mirrors the `*.local.md` overlay pattern but at the persistent XDG layer.

`config.local.yaml` next to `config.yaml` (Option B) was rejected: it would require
per-collaborator copies and doesn't align with how we manage other global paths.

## Schema (initial, minimal)

```yaml
# ~/.config/harnez/local.yaml
# Machine-local overrides — not committed, not managed by harnez apply.

usage:
  default_host: um760      # SSH host used when --host is omitted
  # hosts:                 # optional named aliases (future)
  #   um760: um760.local
```

Only `usage.default_host` is needed for the first implementation. Additional keys
are added on demand. The file is always optional — absence is the same as all
defaults.

## Merge semantics

- `harnez` loads the embedded/specified `config.yaml` first, then merges
  `~/.config/harnez/local.yaml` on top (shallow key-wins override).
- `local.yaml` only affects keys it defines explicitly; unset keys fall through
  to the main config or compiled defaults.
- `harnez status` reports whether a local config is present and its path.
- `harnez apply` never reads or writes `local.yaml`.

## Acceptance Criteria

1. `harnez` respects `$XDG_CONFIG_HOME` if set; falls back to `~/.config`.
2. `harnez usage` (all subcommands: snapshot, watch, summary) reads
   `usage.default_host` and uses it when `--host` flag is absent.
3. `--host` flag always wins over `local.yaml` value.
4. `harnez status` prints a line like:
   `local config:  ~/.config/harnez/local.yaml [present]` or `[absent]`.
5. Absence of the file is not an error; all features degrade gracefully.
6. `go test ./...` passes; the new loader is unit-tested with a temp dir.

## Future uses (not in scope for this ticket)

- `usage.hosts` map for named SSH aliases
- Machine hostname / identity cache (avoid repeated `hostname` calls)
- Per-machine collector interval or history retention override
- Anything else that is static, machine-specific, and not appropriate for `config.yaml`
