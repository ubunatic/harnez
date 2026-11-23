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

## 2. Proposed Solutions

Either or both of:
1. **Tier-aware Quota-1**: allow a full-tier `make test-q1` upgrade after a `-short` run (while still blocking repeat runs at the same or lower tier without intervening source edits).
2. **Docs/skill guidance**: document in `Quota.md` and `lean-sprint` not to run `go test -short ./...` before `make test-q1` when no further edits are planned.

## 3. Acceptance Criteria

- Running `go test -short ./...` followed by `make test-q1` without source edits either succeeds (tier upgrade allowed) or is explicitly documented and guided in `Quota.md` / `lean-sprint`.
- Repeat runs at the same tier without source edits remain blocked.
