# 1275 — exec: Quota-1 blocks full `make test-q1` after `go test -short` without source edits

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1246

---

## 1. Problem & Motivation

Reported from `/home/alfiz/projects/splitty` (`/lean-sprint` across issues 001–007, 2026-11-23):

When a developer agent runs `go test -short ./...` and then immediately runs `make test-q1` (without intervening source edits), Quota-1 blocks `make test-q1` with:

```
test execution blocked because no repository source files have been modified since the last test run
```

Even though `go test -short ./...` skips full-tier PTY/integration tests and `make test-q1` runs the full suite, Quota-1 treats them as the same test execution and refuses the upgrade from `-short` to full.

Current workaround used in `splitty`: run `go vet ./...` followed directly by `HTO=0 make test-q1` (skipping `go test -short ./...` beforehand).

## 2. Decision & Goal

/goal Make Quota-1 tier-aware so upgrading from a narrower/shorter test run (e.g., `go test -short ./...`, `make test-short`, or single-package tests) to a broader/full test run (`make test-q1` / full suite) succeeds without intervening source edits, while repeat runs at the same or lower tier remain blocked, and update `Quota.md` guidance; or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural blocker, or no progress, clear the goal and record a failure report with resume point.

## Decision

- Selected Option 1 + Option 2 (autonomous `/decide`, 2026-11-24): implement tier-aware Quota-1 upgrades in code so `go test -short` -> `make test-q1` works without workarounds, and update `Quota.md` docs to reflect the rule.

## 3. Acceptance Criteria

- Running `go test -short ./...` (or a narrower package/short tier) followed by `make test-q1` without source edits succeeds.
- Repeat runs at the same or lower tier without intervening source edits remain blocked.
- `Quota.md` (and its source/copy if applicable) documents the tier-upgrade behavior.
