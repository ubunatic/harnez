# 205 — `harnez status`/`diff` parity for Codex hooks entry

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Enhancement
**Related**: [[200-codex-native-hooks-preTooluse-wiring]] (added
`internal/codex.Apply` into `ApplyAll`; this ticket's deferral is
documented in 200's "Follow-up" section), [[196-agy-native-hooks-plan-alongside-claude-hooks]]
(made the same scoping deferral for agy's `harnez status` integration)

## Problem

`harnez apply` now writes/merges `~/.codex/config.toml`'s `[hooks.harnez]`
table via `internal/codex.Apply` (and similarly for agy's `hooks.json` via
`internal/agy`), but neither `harnez status` nor `harnez diff` know about
either file. There is no way to preview what `apply` would change to
these hook entries, or to see current install/drift state, without
actually running `apply`.

`internal/codex.Status(path)` and an equivalent status check already exist
for agy — the underlying primitives are there, just not wired into the
`status`/`diff` command surfaces.

## Scope

- Wire `internal/codex.Status` and agy's status equivalent into `harnez
  status`'s output (installed/drifted/absent, matching how other managed
  files are reported).
- Consider whether `harnez diff` should preview the merged TOML/JSON
  before writing (same treatment as other `apply`-managed files).

## Non-goals

- No change to `apply`'s actual merge/write behavior — this is
  visibility-only.
