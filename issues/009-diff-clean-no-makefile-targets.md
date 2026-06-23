# diff and clean don't cover Makefile targets section

**Status:** Open

**Severity:** Low — apply is idempotent; diff/clean gaps are cosmetic for now

## Problem

`DiffAll` and `CleanAll` handle AGENTS.md managed sections but have no knowledge of
the `Language.Targets` field. Two consequences:

1. `claudeconfig diff` never shows changes to the `# claudeconfig:begin targets` block
   in a project Makefile, even if the source (`MakeTargets.mk`) changed.

2. `claudeconfig clean` does not remove the managed targets block from the Makefile.
   Users must edit the Makefile manually to undo the injection.

**Affected:** `internal/claude/apply.go` — `DiffAll` (~line 700), `CleanAll` (~line 730)

## Fix

In `DiffAll`: for each language with `Targets != ""`, call `markdown.DiffMK(dest, "targets", content)`.

In `CleanAll`: for each language with `Targets != ""`, call `markdown.CleanMK(dest, "targets")`.

Both functions already exist (`MKMarkers` added 2026-06-23). Only callers are missing.

`status` should also gain a check: `markdown.ContainsSectionMK(makefilePath, "targets")`.
