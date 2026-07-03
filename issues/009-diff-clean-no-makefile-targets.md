# diff and clean don't cover Makefile targets

**Status:** Open

**Severity:** Low — apply is idempotent; diff/clean gaps are cosmetic for now

## Problem

`DiffAll` and `CleanAll` handle AGENTS.md managed sections but have no knowledge of
`Language.Targets` reconciliation (`internal/claude/maketargets.go`, `ReconcileMakeTargets`).

Since 2026-07-03, target injection is marker-less: it structurally detects "our" targets
(and the ⚙️ `.PHONY` sentinel) instead of relying on a `# claudeconfig:begin targets` HTML/shell
comment block, so users' own Makefile content around it is never clobbered. This makes the
original fix proposal here (`markdown.DiffMK`/`CleanMK` on a "targets" section) obsolete —
there is no single managed block to diff or delete anymore.

Two consequences remain:

1. `claudeconfig diff` never previews what `ReconcileMakeTargets` would do to a project
   Makefile (which targets would be added/updated/prompted/skipped).

2. `claudeconfig clean` cannot undo the injection — a target added by `init` (e.g. `help`)
   stays in the Makefile until removed by hand. Since it's a normal, human-editable target
   with no wrapping markers, this is arguably fine, but is inconsistent with how `clean`
   handles AGENTS.md sections.

**Affected:** `internal/claude/apply.go` — `DiffAll` (~line 700), `CleanAll` (~line 730)

## Possible fix

`DiffAll`: for each language with `Targets != ""`, run `ReconcileMakeTargets` in a dry-run
mode (no writes) and print what would change per target.

`CleanAll`: decide intentionally whether Makefile targets should be cleanable at all — if
yes, remove only targets whose header still carries the sentinel (i.e. still ours,
unmodified by the user); leave sentinel-less (user-edited) targets alone, same rule used to
decide whether to prompt on `init`.
