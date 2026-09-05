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

---

## Implementation Plan

### Scope correction first

`DiffAll` (`internal/claude/apply.go:831`) and `CleanAll` (`:918`) are **global-only** —
they take a `~/.claude` target and have no project dir (`docs/CLIDesign.md:109` records this
explicitly). Makefile reconciliation is a *project* concern owned by `init`
(`internal/claude/init.go:383` → `ReconcileMakeTargets`). Bolting a project Makefile preview
onto `harnez diff` would re-introduce exactly the `apply -p` footgun that `docs/CLIDesign.md`
was written to prevent. So the fix belongs on `init`, not on `DiffAll`/`CleanAll`, and the
"Affected" lines above should be treated as stale.

### Steps

1. **Dry-run mode in `ReconcileMakeTargets`** (`internal/claude/maketargets.go:143`) —
   currently it mixes decision, `fmt.Printf` reporting, and the final write. Split it:
   - Extract the per-target decision loop into
     `planMakeTargets(content, oursContent, cfg) (newContent string, actions []targetAction)`
     where `targetAction{Name string; Kind: added|updated|upgraded|skipped-manual|skipped-diff|unchanged; Reason string}`.
   - `ReconcileMakeTargets` becomes: read file → `planMakeTargets` → prompt where
     `Kind == skipped-diff` and `!assumeYes` → print → write.
   - New `PreviewMakeTargets(dest, oursContent, cfg) ([]targetAction, error)` — same plan,
     no prompting, no write.
   This keeps prompting out of the pure function and makes both the preview and the existing
   test (`maketargets_test.go:15 TestReconcileMakeTargets_Variants`) assert on structured
   actions instead of scraping stdout.

2. **`init --dry-run`** — new flag on `initCmd` (`cmd/harnez/main.go:550`), threaded into
   `RunInit`. When set, `init` reports what it *would* do for every step it already
   performs (AGENTS.md sections, doc copies, Makefile targets) and writes nothing. Makefile
   preview renders the `[]targetAction` from step 1, e.g.:
   ```
   Makefile:
     + help          would add
     ~ install       would update (managed)
     - test          skip (manually defined/customized)
   ```
   A `--dry-run` on `init` is the natural home for consequence 1, is discoverable, and does
   not touch `apply`'s flag surface.

3. **`clean`: decide deliberately — recommendation is *do not* remove Makefile targets.**
   Since 2026-07-03 the injection is marker-less and the emitted targets are ordinary,
   human-editable Make rules; `clean` is global-only and has no project dir to clean anyway.
   Instead:
   - Document the asymmetry in `docs/CLIDesign.md` (one line under the table: "`clean` never
     touches project Makefiles; targets added by `init` are plain Make rules, remove them by
     hand").
   - If a removal path is later wanted, it belongs as `init --clean-targets`, restricted to
     targets whose header still carries the 🤖 managed sentinel (`headerHasSentinel(h, "🤖")`,
     `maketargets.go`) — the same test already used to decide silent updates. Sentinel-less
     (user-edited) targets are left alone. File this as a follow-up ticket rather than
     building it now; nobody has asked for it.

4. **Tests** — `internal/claude/maketargets_test.go`:
   - `TestPreviewMakeTargets` asserting the exact `[]targetAction` for the same fixtures
     `TestReconcileMakeTargets_Variants` uses (missing / managed / manual-small-diff /
     manual-big-diff / legacy-help).
   - Assert `PreviewMakeTargets` leaves the file byte-identical (read before/after).
   - `internal/claude/init_test.go`: `TestRunInit_DryRunWritesNothing` — snapshot the temp
     project dir's file contents before and after `RunInit(..., dryRun: true)`, assert equal
     and that the output names the expected targets.

### Design decisions / tradeoffs

- **`init --dry-run` over extending `diff`** — preserves the apply/init separation, which
  `docs/CLIDesign.md` calls load-bearing. `diff --capture-docs` already covers the one
  legitimate project-facing case on `diff` (managed doc drift) and stays as is.
- **Structured actions over stdout scraping** — the current function is only testable by
  capturing prints; the split pays for itself immediately in step 4.
- **Clean stays out of Makefiles** — reversing a marker-less injection is guesswork, and the
  blast radius (silently deleting a target a user now depends on) is worse than the
  inconsistency being fixed.

### Risks / open questions

- Refactoring `ReconcileMakeTargets` touches the prompt path; `TestReconcileMakeTargets_Variants`
  must keep passing unchanged (it is the regression net for the 2026-07-03 marker-less rewrite).
- `--dry-run` on `init` implies previewing the AGENTS.md and doc-copy steps too; keep that
  minimal (one line per file: `would write` / `unchanged`) rather than emitting full diffs.
- Open: should `init --dry-run` exit non-zero when changes are pending, mirroring
  `diff --exit-code`? Probably yes, as `-e`, for scriptability — decide when implementing.

### Scope

**Medium** — the `planMakeTargets` extraction is the bulk of it (~120 LOC moved/restructured
in `maketargets.go`), plus a flag, plumbing through `RunInit`, and two tests. The `clean`
half is a documentation change only.
