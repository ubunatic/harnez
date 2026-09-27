# 531 — quota-1: a run that stops at gofmt/vet should not use up the single test run

**Status**: Closed — gofmt and vet precheck failures allow one retry
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Tooling
**Related**: [[525-replace-recent-skill-rules-with-harnez-runtime-feedback]], [[530-stats-agents-show-not-0-0-pts-with-no-reliable-turns-label-unreliable-drain-clearly]], `Makefile` (test-q1), `internal/quota1/`

## Problem

In 530 M1 the developer's only `make test-q1` run stopped at the `gofmt -l` check,
so `go test` never ran. After formatting, the rerun was blocked, the commit went in
untested, and the host found a failing test. 524 M1 had a similar waste: the run
happened before any edit.

## /goal

A quota-1 run that fails in format/vet before tests start doesn't count as the test
run (or harnez prints "tests did not run: fix gofmt, then rerun" and allows one
rerun). Runtime feedback, not a new rule.

## Outcome

Quota-1 runs whose output shows the Makefile gofmt or vet gate failed before `go test` now report that tests did not run and allow the existing one incomplete-run retry. Regression coverage exercises both gates. `make test-q1` passed; `make install` completed.
