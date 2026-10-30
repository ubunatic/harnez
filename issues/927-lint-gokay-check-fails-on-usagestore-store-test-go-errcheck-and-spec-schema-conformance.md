# 927 — lint: gokay check fails on usagestore store_test.go errcheck and spec schema conformance

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 924, 919, study `docs/studies/2026-10-25-epic-921-server-monitor-sprint.md`

---

/goal `gokay check` passes its lint and spec stages on main, or each remaining finding is a documented exception;
or stop and report. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Notes

- Reported by developer d919b (919 M4, 2026-10-25): `gokay check` tests passed, but the lint and spec stages failed
  on errcheck findings in `internal/usagestore/store_test.go` (likely from 924, 7f4d55cb) and on schema conformance
  diagnostics. `make lint` and `make test-q1` were green, so the two linters disagree.
- First step: rerun `gokay check`, list the exact findings, and decide whether `make lint` should cover them.

## Findings (job emh7ga4jme7r6q5bjqw4v24gya, 2026-10-30)

Stopped before any code change: the goal needs a policy decision (see below), not a fix.

**What `gokay check` reports on this worktree (base `86a126b0`, clean tree)**

- Default `gokay check` (`--changed` vs `origin/HEAD`): `fmt`, `vet`, `lint`, `spec`, `test` all `skipped-empty`. Nothing changed, so nothing is checked.
- `gokay check --all`:
  - `fmt` ok, `vet` ok.
  - `lint` `skipped-missing-tool`: `golangci-lint` is not on `PATH` on this host.
  - `spec` `skipped-empty`, even with `--all`. The schema-conformance diagnostics from d919b do not reproduce.
  - `test` fails: `internal/readcard` `TestGoldenFontAssets` reports 3x5, 5x8, 6x12, 7x13, 8x16 golden assets as stale. This is outside this ticket's scope and is pre-existing on a clean checkout.

**What golangci-lint v2.15.0 reports (installed to the job's GOBIN, default linters, caps disabled)**

- `internal/usagestore`: 35 errcheck. 16 in `store_test.go`, 19 in `store.go`. All are unchecked `Close` / `Rollback` / `rows.Close`.
- Repo-wide `./...`: 1261 issues (errcheck 1074, staticcheck 133, unused 42, ineffassign 12).
- golangci's default `max-same-issues: 3` hides repeats, so the first run undercounts. Use `--max-same-issues=0 --max-issues-per-linter=0` for exact counts.

**Attribution check**

- The ticket says the findings are "likely from 924 (7f4d55cb)". Not supported. The 924 block adds one of the 16 `store_test.go` findings (`defer s.Close()` at line 271). The same unchecked-`Close` pattern appears in tests from before 924 (lines 19, 45, 83, ...), so it is a repo-wide convention, not a 924 regression.

**Decision needed (cannot be taken from the ticket or the code)**

`make lint` runs gofmt, go vet, `harnez lint --check` and `scripts/lint.sh`. It does not run golangci-lint, and the repo has no `.golangci.yml`. Adopting golangci's default set makes the gate red with 1261 findings. Options:

1. Wire golangci into `make lint` and add a `.golangci.yml` that excludes errcheck on `Close`/`Rollback`. Then fix the remaining staticcheck / unused / ineffassign findings. Needs an owner decision on the exclusion list and a sweep of about 190 non-errcheck findings.
2. Keep `make lint` as is. Treat `gokay lint` (changed lines only) as the advisory gate for new code. Document the exception in the ticket or in `docs/`. Nothing is fixed here.
3. Fix all 1261 findings. Not recommended for a P3 ticket.

Recommendation: option 1, with the errcheck exclusion limited to `Close` and `Rollback`, since those dominate the count and are idiomatic in tests. This still needs an owner decision.

**Spec stage**

The spec-conformance diagnostics cannot be reproduced. gokay's spec step is `skipped-empty` even with `--all`. The `validate-spec` binary on this host fails with `spec/schemas/version.schema.json: no such file`, and `make` has no `validate-spec` target. Resume point: get the d919b log or the exact gokay version they used, then rerun.

**Resume point**

1. Owner picks option 1, 2 or 3 above.
2. If option 1: add `.golangci.yml`, run `golangci-lint run --max-same-issues=0 --max-issues-per-linter=0 ./...`, then fix or document the rest. Install golangci-lint on PATH first (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`, v2.15.0 used here).
3. Get the spec-stage reproduction from d919b, then rerun `gokay check --all`.
4. Separately: `TestGoldenFontAssets` in `internal/readcard` needs `go run ./scripts/generate-golden-fonts.go`. Track it in its own ticket.

Working files for this report: `/tmp/gokay-all-927.log`, `/tmp/gl-usagestore-full-927.log`, `/tmp/gl-all-full-927.log`.
