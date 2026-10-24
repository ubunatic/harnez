# 907 — tests: import instead of copy, sunset covered canaries, isolated canaries become container tests

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Testing
**Related**: 895 (lint for hardcoded copy lists, closed in favour of this), 889 (`docs/Canary.md` "Sandbox Copy Fixtures")

---

/goal No test copies repo code into a temp module: each copy fixture is replaced by an import, removed as a
canary already covered by regular tests, or turned into a container test; `docs/Canary.md` states the three
rules. Stop and report when a fixture fits none of the three. Circuit breaker: on a loop or no progress, clear
the goal and write a failure report here.

## 1. Problem & Motivation

Copy fixtures (tests that copy part of the repo into a temp module) break when packages move: a hardcoded
copy list misses the new package and the test fails in the wrong place. Linting the lists (895) treats the
symptom. Owner decision (2026-10-24): avoid copying code at all.

## 2. Rules

1. **Import, don't copy.** Code a test needs goes into an `internal/` package and is imported.
2. **Sunset covered canaries.** A canary whose behaviour the regular tests already cover is removed; mature
   projects with broad tests need few canaries.
3. **Isolation means a container test.** A canary that must prove something works away from the host becomes
   a container test with a minimal image holding only what it needs.

## 3. Plan

1. Replace the "Sandbox Copy Fixtures" section in `docs/other/Canary.md` (source) with the three rules; sync
   the root copy.
2. Audit the copy fixtures, one verdict each (import / sunset / container), recorded here:
   `internal/claude/integration_test.go`, `internal/claude/init_phases_test.go`,
   `internal/subagent/driver_test.go`, `internal/finder/search_test.go`, `internal/exec/exec_test.go`
   (candidates from a grep for copy/sandbox helpers; verify each actually copies repo code).
3. Convert per verdict, one milestone per verdict group. Verify with `make test-q1` and, for container
   tests, `make container-test`.

## 4. Audit Results

No real copy fixture was found, so no import/sunset/container conversion is needed.

- **Clean:** `internal/claude/integration_test.go`, `internal/claude/init_phases_test.go`,
  `internal/subagent/driver_test.go`, `internal/finder/search_test.go`, and
  `internal/exec/exec_test.go` (the five files named in the plan); their temporary files are
  test inputs or outputs, not copies of repository code.
- **Clean:** `internal/claude/lite_doc_structural_gate_test.go:71-95` reads full/lite docs as
  input; it does not copy or build code. `cmd/harnez/telemetry_writer_boundary_test.go:12-76`
  walks and parses repository Go files in place; it does not copy them.
- **Clean:** `internal/exec/stopped_test.go:48-57` reads `stopped.go` to check a source constraint
  in place. `scripts/gotest-summary/main_test.go:12-30` writes synthetic Go test input, not
  repository source.
- A repository-wide `*_test.go` scan for temp directories, file reads/writes, copy helpers,
  package walks, and `go.mod` creation found no other test copying repository code into a
  temporary module/directory. No fixture fits the three conversion verdicts because there are
  no real fixtures.
