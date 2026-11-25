# 1298 — Integrate fix/init-quota-off (spool pointers, quota-off init) onto main after runner-mode removal; drop the obsolete confinement work

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Chore
**Related**: #1287 (quota-off init), #1289 (retire runner contract), #1280 (taia sandbox access); upstream 859d787b (remove runner mode), 565fd744 (Quota-1 advisory, no sandbox), c4744476 (rules header without Quota.md), 7cba5775 (access records under store override), 82e25c0a (socktest)

---

## 1. Goal

/goal Land the still-useful work of local branch `fix/init-quota-off` on `main` through one AGit PR, rebased on current `origin/main`, with the obsolete confinement work dropped and the unstaged probe-confinement changes discarded after a backup. Done when the PR is merged, `make install`, `make lint` and one `HTO=0 make test-q1` pass on the rebased branch, and #1287 is closed or updated with what remains. Stop and report when an owner decision below is unanswered or a push is refused. On a loop, a hard blocker or no progress, clear the goal and write a failure report (tried, why it failed, resume point) here.

## 2. Findings (2026-11-25, branch at ad261e05, origin/main at 7ab9a5fd, merge base 4c8c4bb9)

The branch has 14 commits; main gained 43. Main removed runner mode and the whole `internal/runstate` package, made Quota-1 advisory and deleted the bwrap/Landlock sandbox, and added `HARNEZ_STORE_DIR`. Its principle (859d787b): harnez must not branch on who calls it. A straight merge conflicts in 10 files.

| Commits | Verdict | Why |
|---|---|---|
| 571b0b69, f961a419 (HARNEZ_STORE_DIR, drop runner guard) | drop | superseded by 859d787b; keep only the error text naming `--store-dir` and `HARNEZ_STORE_DIR` if main lacks it |
| 4a777882, e465a670, 8eda0fe4, ec07b59f (spool pointers, list, wait) | keep | trial cherry-pick onto main: only conflict is `internal/subagent/runner_registry_test.go` (deleted on main); after dropping that file `go build ./...` and `go vet` pass. Move the spool-write assertions into a new test file. No ticket or doc covers this feature yet |
| 52b3951e (config --access store and spool rows) | keep | picks cleanly; already drops the mode probe, in line with main |
| 35e72cc6, 799a3d86, f2b40bda, 92bdfb3e (bwrap/Landlock fixes) | drop | the sandbox no longer exists on main (565fd744) |
| bf30589e (runstate canary probe for confinement) | drop | main deleted `runstate`; a write-probe that changes behaviour contradicts "do not branch on the caller" |
| a7d2c765, ad261e05 (quota-off init) | rework | the rules-header part duplicates c4744476 (conflicts in `init_agent_file.go`, `rules_init_test.go`, `quota_off_init_test.go`); keep `quota_text.go` (test-q1 → make test rewrite), the optional `rules` block in the repo schemas, the `docs/Spec.md` + `docs/other/Spec.md` wording and the scan-all-docs test, which cover the rest of #1287 |

**Unstaged work (44 files, +525/−1173, 4 untracked files, 2 renames):** it continues bf30589e. It turns `runstate` into a probe-based `IsConfined` per canary dir, removes runner branches (man, install, promote, modelblock, tier overrides, usage, procs, telemetry spool), and adds `TestMain` HOME isolation plus short socket-dir helpers. Main already removed the same runner branches differently, and `internal/testutil/socktest` covers the socket-dir helpers. Nothing in it should be ported except as a reference for removals main may have missed; check those against #1289.

## 3. Plan

1. Back up: `git stash push -u -m "probe-confinement WIP"` (or commit it to a local `wip/probe-confinement` branch), then confirm a clean tree.
2. New topic branch from `origin/main`. Cherry-pick 4a777882 (drop the deleted test file; re-home its spool assertions), e465a670, 8eda0fe4, ec07b59f, 52b3951e.
3. Port the non-header part of a7d2c765/ad261e05 by hand on top of c4744476. Then `make install`, `harnez init -d .`, `git checkout --` unrelated rewrites, and commit source and copies together (`TestRootDocCopiesMatchSources`).
4. Document the spool feature: `HARNEZ_SPOOL_DIR` in `docs/CLIDesign.md` or the session-isolation doc, and a spec entry if the default path is configurable (`docs/Spec.md`).
5. `make install`, `make lint`, one `HTO=0 make test-q1`. Fresh reviewer subagent (feature-sized), then an AGit PR and `harnez pr wait`.
6. Delete the old branch and the stash only after the owner confirms.

## 4. Owner decisions (open)

- Keep the spool-pointer feature now that main has `HARNEZ_STORE_DIR`? Its use case is a host listing and waiting on sessions that a sandbox wrote to its own store. Recommended: keep it.
- a7d2c765 makes `rules.quota` default to false when the `rules` block is absent. Confirm that this default matches c4744476 and 92cfb7df on main.
- Discard the unstaged probe-confinement WIP after the backup? Recommended: yes.
