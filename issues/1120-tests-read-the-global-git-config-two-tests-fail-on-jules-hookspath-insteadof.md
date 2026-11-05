# 1120 — Tests read the global git config: two tests fail on Jules (hooksPath, insteadOf)

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: reports/jules/env-check.md

---

/goal Make the git-using tests independent of the user's global and system git config, so `make test-short`
passes in the Jules VM; or stop and report when blocked on a user decision.

## 1. Problem & Motivation
The Jules env-check run (2026-11-04, `reports/jules/env-check.md`, commit 8340dd45) had `make test-short`
fail in two tests that pass locally:
- `TestIssuesRebaseRetryAfterFailedRepairCommit` (`internal/issues/maintenance`)
- `TestReleasePushSkipsRemoteCoveredByOthers/fan-out-refused` (`internal/release`)

Jules's VM setup sets global git config before our script runs:
`core.hooksPath /dev/null` and `url.http://git@192.168.0.1:8080/.insteadOf https://github.com/` (and
`git@github.com:`). Any CI or developer machine with a global hooksPath or insteadOf can hit the same.

## 2. Findings (unverified hypotheses)
- hooksPath=/dev/null disables hooks; a test that forces a commit to fail through a hook then passes
  the commit and fails its assertion.
- insteadOf rewrites github.com remote URLs; the fan-out test's refused push may change shape.

## 3. Plan
1. Reproduce locally: `GIT_CONFIG_GLOBAL=<file with those two settings> go test -run <name> ./<pkg>`.
2. Isolate git in tests: a shared test helper (or TestMain) sets `GIT_CONFIG_GLOBAL=/dev/null` and
   `GIT_CONFIG_NOSYSTEM=1`, with identity set per repo. Apply it to every package that runs git.
3. Verify with step 1's config file: both tests pass.

## Resolution 2026-11-05
Reproduced: both failures came from `core.hooksPath=/dev/null` (the refusing hook never ran).
`internal/gittest.Isolate` points `GIT_CONFIG_GLOBAL` at a temp file holding only a test identity and
`init.defaultBranch=main`, and disables the system config. Every package whose tests or code under test run
git calls it from `TestMain` (19 packages, incl. `cmd/harnez`, `internal/lint`, `internal/prwait`).
`go test -short ./...` passes with a Jules-like global config (hooksPath, insteadOf) and with the normal one.
Test: `TestIsolateHidesGlobalHooksPath`. Review by agy:sonnet:med was cut off by its quota; test-only change,
committed as a small fix. A new git-using package must add the same `TestMain`.
