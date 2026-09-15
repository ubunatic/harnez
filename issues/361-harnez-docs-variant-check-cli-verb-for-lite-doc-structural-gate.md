# 361 — harnez docs variant --check CLI verb for lite-doc structural gate

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Feature
**Category**: CLI / Templates
**Related**: [Issue 359](359-pilot-agenticloop-lite-md-behavioral-canary-gate.md) (pilot that used a
stopgap `go test` instead of this verb), [Issue 360](360-harnez-docs-variant-name-lite-full-thin-switch-verb.md)
(adds the `harnez docs variant` command family this extends)

---

## 1. Problem & Motivation

Issue 359's acceptance criteria assumed a `harnez docs variant --check <name>` CLI verb that
asserts every bolded rule-name / `###` heading in a doc's full source has a corresponding entry
in its lite source (catches omission, not wrongness). That command does not exist. The pilot
shipped a stopgap instead: `internal/claude/agenticloop_lite_test.go`'s
`TestAgenticLoopLiteStructuralGate`, a `go test` hardcoded to `docs/practices/AgenticLoop.md` /
`AgenticLoop.lite.md`.

The stopgap works for the pilot but doesn't generalize — it can't check any other doc entry that
gains a `lite_source`, and it isn't reachable outside `go test`.

## 2. Proposed Fix

Generalize `TestAgenticLoopLiteStructuralGate`'s heading/bold-name extraction and diff logic into
a reusable function, then expose it as `harnez docs variant --check <name>`:
- Resolve `<name>`'s `source` and `lite_source` from `config.yaml` via the existing `Language`
  struct (issue 357).
- Extract `### ` headings and `**bold**` rule-names from the full doc.
- Report any that don't appear (case-insensitive substring) in the lite doc.
- Exit non-zero on any omission; print the missing list.
- Keep (or replace) the existing Go test as a regression guard for the AgenticLoop pilot
  specifically — the CLI verb should not remove test coverage, just add reusable tooling.

## 3. Acceptance Criteria

- [ ] `harnez docs variant --check <name>` implemented, using config.yaml's `source`/`lite_source`
      for the named doc entry.
- [ ] Errors clearly if the entry has no `lite_source` set.
- [ ] `TestAgenticLoopLiteStructuralGate` refactored to call the same shared extraction/diff logic
      (no duplicated regex/logic between the CLI verb and the test).
- [ ] `go test ./...` and `scripts/smoke-test.sh` pass.
