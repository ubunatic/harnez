---
kind: sprint
ticket: 1206
date: 2026-11-19
branch: sprint/taia-harnez-pack
---

# Two-Agent Lean Sprint: 1206 — AGit `git pull` Ergonomics & Push-to-Main Policy

## 1. Scout Audit (`search-1206`, `claude:haiku:med`, 1m5s)
- Mapped `internal/gitflow/gitflow.go` (`GuardCommit`, `GuardRelease`, `spec/git.yaml`), `internal/claude/init.go` (`git config --local merge.*`), `scripts/git-hooks/pre-push` (`internal/githooks`), and `docs/lang/Git.md` + `docs/practices/AGit.md`.
- Confirmed neither `branch.autoSetupMerge` nor `checkout.defaultRemote` was set anywhere, and raw `git push` lacked a `git.protected_main` guard.

## 2. Milestones & Review
- **M1 (`96f338d8`)**: `dev-1206` (`claude:haiku:high`, 12m41s, 7.06M tokens incl. cached) added repo-local AGit pull configuration (`internal/claude/init_agit.go`) triggered by `git.mode: team`, Forgejo `spec/pr.yaml` `origin` URLs, or `refs/for/` push refspecs; added `gitflow.GuardPush`, hidden `harnez push-guard`, `scripts/git-hooks/pre-push` integration, unit/CLI/hook tests, and synced `docs/lang/Git.md` + `docs/practices/AGit.md` (+ root `docs/Git.md` & `docs/AGit.md`). `make lint` and `HTO=0 make test-q1` passed.
- **Cross-vendor Review (`rev-1206`, `claude:opus:med`, 1m23s, fallback on AGY 5h 88%)**: Returned **FAIL** catching two high-severity defects (remote-blind `GuardPush` blocking GitHub `main` mirrors & `harnez release` mirroring, and false "`harnez release` is not blocked" doc line) plus stale `harnez` binary handling in `pre-push`, bare-remote `origin/main` tracking pull test gap, and `echo` vs `printf` Bash style.
- **Refinements (`54ee3cf9`)**: `dev-1206-r2` (`claude:haiku:high`, 10m8s, 611k tokens incl. cached) added `GuardPushTarget(dir, remote, url, ref)` + `--remote`/`--url` flags on `push-guard`, checked `harnez push-guard --help` for `--url` in `pre-push`, added `TestPrePushReleaseLegs`, `TestPrePushStaleHarnez`, `TestInitAgitPullFollowsOriginMain` (bare remote `origin/main` + `git pull`), switched to `printf`, updated `help.golden` + Git/AGit docs, and passed `make lint` + `HTO=0 make test-q1` (disclosing `internal/githooks` skipped without `git-lfs`).
- **Re-review (`rev-1206-r2`, `claude:opus:med`, 36s)**: Returned **PASS WITH MINOR FINDINGS**.
- **Host Verification**: `make install`, `make lint` (`0 issues.`, `lint: ok`), `HTO=0 go test ./internal/claude/... ./internal/gitflow/... ./cmd/harnez/...` (`ok` all 3), AND un-skipped `PATH="<tmp-git-lfs-shim>:$PATH" HTO=0 go test -v -count=1 ./internal/githooks/...` (`ok 3.290s`, all 17 subtests including `TestPrePushReleaseLegs`, `TestPrePushStaleHarnez`, `TestPrePushGuard`, and `TestPrePushPushPolicy` passed).

## 3. Outcomes
- `harnez init` idempotently configures repo-local `branch.autoSetupMerge=always` and `checkout.defaultRemote=origin` on AGit repos (including `tracker: none` + Codeberg `origin` repos like `taia`) without touching non-AGit repos or global git config.
- `gitflow.GuardPushTarget` + `harnez push-guard` + `scripts/git-hooks/pre-push` refuse direct default-branch pushes on `protected_main: true` AGit remotes while keeping `solo`, `protected_main: false`, GitHub mirrors, `refs/for/...`, topic branches, and `/disable repo-rules push-main` working smoothly.
