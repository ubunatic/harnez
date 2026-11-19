---
kind: sprint
ticket: 944
date: 2026-11-19
branch: sprint/taia-harnez-pack-2
---

# Two-Agent Lean Sprint: 944 — Count Targeted & Coverage `go test` Runs Under Quota-1

## 1. Scout Audit (`search-944`, `claude:haiku:med`, 1m15s)
- Found `Quota.md` rules duplicated across `config.yaml:765-772` (`quota_section`) and `config.yaml:905-914` (`Quota.md` rule template), plus `lean-sprint.md` (§3) and `lean-sprint-v1.md` (**§2**, correcting ticket's §3 citation).
- Mapped `detectQuota1` (`internal/exec/exec.go:146-159`), `quota1.CheckAndRecord` (`internal/quota1/quota.go`), and noted why top-level `bash -c` rewrites and nested child `go test` invocations inside `make test-q1` must be handled carefully.

## 2. Milestones & Review
- **M1+M2 (`46948f89`, cherry-picked onto merged `main` as `c60cee14`)**: `dev-944` (`claude:haiku:high`, 9m23s, 535k tokens incl. cached) updated `config.yaml` (`quota_section` + `Quota.md`), `.harnez/rules/Quota.md`, `lean-sprint.md`, `lean-sprint-v1.md`, added shell-segment detector `internal/exec/quota1_command.go`, prevented recursive counting via `HARNEZ_QUOTA1_ACTIVE=1`, gated detection on `quota1Configured`, and added all 5 locked test scenarios + detector table. `make lint` and `HTO=0 make test-q1` passed.
- **Cross-vendor Review (`rev-944`, `claude:opus:med`, 1m54s)**: Returned **PASS WITH MINOR FINDINGS** (noted plain `go test` unconditionally entering `bwrap` sandbox on Linux/macOS, `env`/`timeout`/`nice`/`time` flag-value skipping, `config.yaml` vs `.harnez/rules/Quota.md` sync drift, and missing unit tests for `rules.quota` and `quota_1.state` triggers).
- **Refinements (`e8d834df`)**: `dev-944-r2` (`agy:neon:med`, 12m21s, 2.43M tokens incl. cached) separated `detectExplicitQuota1` (`bwrap` sandbox only for explicit `--quota-1`/`HARNEZ_QUOTA_1=1`) from `quota1Active` counting/gating, extended `skipCommandPrefix` for `env`/`timeout`/`nice`/`time` options, aligned `config.yaml` + `.harnez/rules/Quota.md` (`./harnez init -y -d . -> No changes.`), added tests for all 3 `quota1Configured` triggers, and passed `make lint` + `HTO=0 make test-q1` (`132/132` Pi tests, `PASS` stage install, `cmd/harnez` `68.511s`, `internal/exec` `5.835s`, `internal/claude` ok).
- **Re-review (`rev-944-r2`, `claude:haiku:high`, 1m20s)**: Returned **PASS WITH MINOR FINDINGS**.
- **Host Verification**: `make install`, `make lint` (`0 issues.`, `lint: ok`), and `./harnez init -d .` (`No changes.`, clean tree) all passed.

## 3. Outcomes
- Plain, targeted (`-run`), coverage (`-cover` / `-coverprofile`), env-prefixed, and compound `go test` / `gotestsum` runs now count toward the Quota-1 single-test budget in repos using Quota-1, without forcing `bwrap` sandboxing outside explicit `--quota-1` runs and without double-counting nested test processes inside `make test-q1`.
- Rule sources (`config.yaml` `quota_section` + `Quota.md`), `.harnez/rules/Quota.md`, and `lean-sprint` / `lean-sprint-v1` docs explicitly document single-budget counting and passing `-coverprofile` through `make test-q1`.
