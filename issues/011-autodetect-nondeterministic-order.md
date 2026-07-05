# autoDetectDocs non-deterministic order

**Severity:** Low — cosmetic only, no functional impact

## Problem

`autoDetectDocs` in `init.go` iterates over `cfg.AgentsMD.Languages`, which is a Go map.
Map iteration order is randomised per run, so the order in which auto-detected docs are
appended to the list (and therefore the order of entries in the generated `Language
Conventions` section of `AGENTS.md`) is non-deterministic.

This causes `init` to be non-idempotent in the visible output — the section content is the
same but the line order may differ between runs, producing spurious diffs.

## Fix

Sort the detected names before appending, e.g.:

```go
sort.Strings(detected)
return detected
```

Or, if a stable preferred order matters, define it explicitly in `config.yaml` (the `docs:`
list already has a user-defined order) and iterate that list instead of the map.

## Status

Open — not yet started.
