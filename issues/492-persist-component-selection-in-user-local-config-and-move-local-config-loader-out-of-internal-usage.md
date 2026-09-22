# 492 — Persist component selection in user-local config and move local config loader out of internal/usage

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Architecture
**Related**: 491 (depends on), 490, 109 (local config), `docs/HarnezComponents.md` §8.5

## /goal

Let a machine keep its component selection (e.g. "no telemetry on this box") across plain
`harnez apply` runs, via `components:` in `~/.config/harnez/local.yaml`, written by
`harnez apply --components … --save`.

## 1. Problem & Motivation

A `--components` flag alone is not persistent: the next plain `apply` reinstalls
everything and `diff`/`status` report the deselected parts as drift. The user-local config
from issue 109 is the right home for a machine-local choice, but its loader
(`usage.LoadLocalConfig`) lives in `internal/usage`, which is moving to a `../loom` app.
It is also the source of the `claude → usage` import edge (`docs/HarnezComponents.md` §2.1).

## 2. Technical Specification

- Move `LocalConfig`, `LocalConfigPath`, `LoadLocalConfig` into a neutral package
  (e.g. `internal/localconfig`); `internal/usage` and `internal/claude` import it.
- Add `components: []string` to the local config; resolution order stays
  flag > config.yaml > local config > full (`components.Resolve` already models it).
- `apply --save` writes the resolved selection, preserving other local config keys.
- Coordinate with the loom move: the usage sections stay readable by whichever tool owns
  them after the move.

## 3. Implementation & Verification Plan

- [ ] Extract the loader; no behaviour change (existing usage/status tests pass)
- [ ] `components:` key and `--save`
- [ ] Test: saved selection is honoured by plain `apply`, `diff`, `status`
