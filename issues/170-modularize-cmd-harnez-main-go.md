# 170 — Modularize `cmd/harnez/main.go` Command Definitions

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Code Quality / Refactor
**Related**: `cmd/harnez/main.go`, `cmd/harnez/find.go`, `cmd/harnez/exec.go`, `cmd/harnez/distill.go`, `cmd/harnez/rate.go`

---

## 1. Problem & Motivation

`harnez assess` flags `cmd/harnez/main.go` as a high-LOC (>500 LOC) file:
```
cmd/harnez/main.go: High LOC file (>500 LOC, consider splitting/modularizing)
cmd/harnez/main.go: High token weight (>4k tokens, agent context heavy)
```

While newer subcommands (`find`, `exec`, `distill`, `rate`, `repostatus`, `mode`, `stats`) live in their own dedicated files under `cmd/harnez/`, older commands (`usage`, `apply`, `diff`, `clean`, `assess`, `scan-docs`) and their respective helper logic remain embedded directly within `main.go`.

## 2. Technical Specification

Decompose `cmd/harnez/main.go` by extracting top-level command declarations and their local helpers into dedicated files in `package main`:

1. **`cmd/harnez/usage.go`**:
   - `usageCmd`, `agentCollectorCmd`
   - Usage flag parsing/validation helpers (`validateUsageFlags`, `resolveUsageHost`, etc.)
2. **`cmd/harnez/apply.go`**:
   - `applyCmd`, `diffCmd`, `cleanCmd`
3. **`cmd/harnez/assess.go`**:
   - `assessCmd`, `scanDocsCmd`
4. **`cmd/harnez/main.go`**:
   - Retain only `rootCmd`, `init()`, version flags, and core execution entry point `main()`.

### Constraints
- **Zero behavioral changes**: Flags, exit codes, help text, and default behaviors must remain byte-for-byte identical.
- Keep all new files in `package main` so internal command references and variables remain accessible without cyclical dependencies.

## 3. Verification Plan

1. Run `make test` across the project.
2. Run `harnez assess harnez` to verify that `cmd/harnez/main.go` no longer triggers the high-LOC warning.
3. Run `make install` and verify CLI subcommands (`harnez usage -h`, `harnez apply -h`, `harnez assess -h`).

---

## 4. Implementation Plan

### Key finding that shapes the whole refactor

Every command in `main.go` is declared as a **local variable inside `main()`**, closing over two
shared vars: `configPath` and `target` (declared at `cmd/harnez/main.go:125-126`). Those two vars
are bound by `StringVarP` into **five separate commands' flag sets** — `apply` (439/440), `diff`
(486/487), `scanDocs` (510), `clean` (525/526), `status` (541/542). It works only because exactly
one command runs per process.

So this is not a cut-and-paste move. Each extracted command must become a `newXCmd()` constructor
that declares its **own** `configPath`/`target` locals, matching the pattern the newer files already
use (`newFindCmd`, `newRateCmd`, `newStatsCmd`, …). That removes the shared mutable state as a side
effect and is behaviour-identical, since no invocation ever reads another command's binding.

### Order of work (each step compiles and passes tests on its own)

1. **`cmd/harnez/apply.go`** — extract `apply`, `diff`, `scanDocs`, `clean`, `status` as
   `newApplyCmd()`, `newDiffCmd()`, `newScanDocsCmd()`, `newCleanCmd()`, `newStatusCmd()`
   (main.go:426-547). Do this first: it is the densest cluster and the one carrying the shared-var
   hazard, so landing it first retires the risk. Note the ticket's §2 grouping puts `scanDocs` in
   `assess.go`; put it here instead — it shares `claude.OpenConfig(configPath)` and the `-c` flag
   shape with `apply`/`diff`/`clean`, and shares nothing with `assess`.
2. **`cmd/harnez/usage.go`** — extract `usageCmd` (144-268), `loadStreamCmd` (271-282),
   `historyCmd` + its four subcommands (283-384), and `collectorCmd` (389-421) as `newUsageCmd()`,
   `newLoadStreamCmd()`, `newHistoryCmd()`, `newCollectorCmd()`. Move `resolveUsageHost` (88) and
   `validateUsageFlags` (108) here too — their tests live in `cmd/harnez/usage_test.go` already, so
   the test file needs no change. This is the largest single chunk (~280 lines) and the biggest LOC
   win.
3. **`cmd/harnez/assess.go`** — extract `assessCmd` (575-601) as `newAssessCmd()`.
4. **`cmd/harnez/initcmd.go`** — extract `initCmd` (549-573) as `newInitCmd()`. The ticket's §2 does
   not list `init`, but it is ~25 lines with 9 flags sitting in `main()` for no reason; leaving it
   is the only thing that would keep `main.go` cluttered after steps 1-3. Name the file `initcmd.go`
   rather than `init.go` to avoid confusion with Go's `init()` function.
5. **`cmd/harnez/main.go`** — should then retain only: imports, `sessionTipHook` (30),
   `main()` with the `root` command declaration, the single `root.AddCommand(...)` call rewritten to
   call the new constructors, and `root.Execute()`. Target ~60 lines.

### Verification (per step, not just at the end)

- `go build ./... && go test ./...` after each of steps 1-4.
- Capture `harnez <cmd> --help` output for all affected commands **before** starting (into the
  scratch dir), and diff against post-refactor output at the end — this is the only mechanical proof
  of the §2 "byte-for-byte identical help text" constraint. Cobra orders `AddCommand` children by
  name in help, so preserving the argument order in the single `root.AddCommand(...)` call is not
  required, but preserving the *set* is.
- `harnez assess harnez` at the end: confirm `cmd/harnez/main.go` no longer trips the >500 LOC /
  >4k token warnings, and confirm none of the *new* files trip them (usage.go will be ~290 lines —
  fine, but check).
- `make install` last, per repo convention.

### Key Decisions / Tradeoffs

- **Constructor functions, not package-level `var`s**: matches the existing newer-file convention
  and keeps flag state per-invocation rather than package-global. Slightly more boilerplate than
  moving the vars to package scope, but package-scope vars would preserve exactly the shared-state
  hazard this refactor should retire.
- **Regrouping `scanDocs` away from `assess`**: deliberate deviation from §2's grouping, justified
  by shared config-flag shape. Note it in the commit message so the ticket/commit disagreement is
  intentional and visible.
- **No behaviour changes bundled in**: resist fixing anything noticed in passing (e.g. the duplicate
  `--proc`/`--processes` binding at 266-267). File separately if it matters.

### Risks / Open Questions

- Low risk overall — it is mechanical, compiler-checked, and the constraint is checkable by diffing
  help output. The one real hazard is a flag silently changing default or shorthand during the
  retyping of the `StringVarP` calls; the captured-help-diff step is what catches that, so do not
  skip it.
- `harnez assess`'s threshold may still flag `cmd/harnez/exec.go` (563) and `exec_test.go` (572).
  Out of scope here; do not expand.

### Scope

**Medium** — ~600 lines relocated across 5 files, zero new logic, but four discrete verification
passes.
