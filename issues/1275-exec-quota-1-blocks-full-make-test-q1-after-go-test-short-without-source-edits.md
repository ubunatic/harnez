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

## 4. Architect Refinement (`argon:med`, 2026-11-24) — M1

- **Tier model (`internal/quota1/quota.go`)**:
  - `TierPackage = 1` (`go test ./pkg`, `go test -run Foo ./...`, `make test-pkg`), `TierShort = 2` (`go test -short ./...`, `make test-short`), `TierFull = 3` (`make test-q1`, `make test`, `make check`, `go test ./...`, or unrecognised explicit `--quota-1` command).
  - Add `Tier int` (`json:"tier,omitempty"`) to `RunRecord` and `Tier int` to `CheckOptions`; normalize `0` via `effectiveTier(t) -> TierFull` for legacy/default backwards compatibility.
  - In `CheckAndRecord`: when `len(modifiedFiles) == 0`, allow if `currentTier > effectiveTier(previous.Tier)` (record new `RunRecord` with `Tier: currentTier`, `Retry: false`, message `"Quota-1: tier upgrade (%s -> %s) allowed; quota recorded"`); block when `currentTier <= effectiveTier(previous.Tier)`. Keep unfinished-run retry/block logic (`Exit == nil && Finished == nil`) intact before the tier check.
- **Command classifier (`internal/exec/quota1_command.go` + `internal/exec/exec.go`)**:
  - Add `classifyQuota1Tier(args []string) int` taking the max tier across simple command segments (`go test` / `gotestsum`, `make` targets, fallback `TierFull`), and pass `Tier` into `quota1.CheckOptions` in `runExecWrapper`.
- **Docs & Copies**:
  - Update `config.yaml` (`quota_section` and `Quota.md` rule template, preserving anchor phrases tested by `internal/claude/quota1_init_test.go`), sync `.harnez/rules/Quota.md` via `make install && harnez init -y -d .`, and note tier upgrades in `docs/commands/lean-sprint.md` and `docs/commands/lean-sprint-v1.md`.
- **Tests**:
  - `internal/quota1/quota_test.go`: `TestCheckAndRecord_TierUpgradeAndBlock` (upgrades allowed; same/lower/legacy blocked; post-edit downgrade resets tier).
  - `internal/exec/exec_test.go`: `TestClassifyQuota1Tier`, end-to-end `runExecWrapper` (`go test -short ./...` -> `make test-q1` succeeds; repeat `make test-q1` or downgrade blocked), and update `TestRunExecWrapper_PlainGoTestRecordsQuota1State` (`:1936-1973`) to reflect same-tier block + upgrade to `make test-q1` allowed + repeat `make test-q1` blocked.

## 5. Delivery & Review (M1)

- **M1 delivered (`01fee008`)**: Implemented `TierPackage`/`TierShort`/`TierFull` in `internal/quota1`, `classifyQuota1Tier` in `internal/exec/quota1_command.go`, wired into `runExecWrapper`, updated `config.yaml`, `.harnez/rules/Quota.md`, `docs/commands/lean-sprint*.md`, and added unit + wrapper tests.
- **Review (`codex:sol:med`, 1m58s) — Needs pre-work (folded into M2 pre-work)**:
  1. `[MED]` `quota1_command.go`: remove `buildvcs` from `goTestValueFlags` (it is boolean in `go test`).
  2. `[MED]` `quota1_command.go`: recognize `-test.short`, `-test.run`, `-test.bench`, `-test.fuzz` aliases.
  3. `[MED]` `.harnez/rules/Quota.md`: deduplicate lines 3-4 vs 13-14 so only the tier-aware rules remain.
  4. `[LOW]` `classifyMakeArgs`: allow decimal load averages (`-l 1.5`).
  5. `[LOW]` Add unit tests for those parser cases and unfinished Package -> Full blocking.
