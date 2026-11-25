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

## 4. Decision (owner, 2026-11-25)

- Keep the spool-pointer feature.
- Discard the probe-confinement WIP. Backup: `git stash list` entry "1298 backup: probe-confinement WIP from fix/init-quota-off" (host clone only). Delete it after this ticket closes.
- Developer: `haiku:high`. Host skips the Searcher because the findings above were checked against live code in this session (trial cherry-pick, build, vet). The developer's first turn is plan-only.
- Sprint branch: `sprint/1298`, based on cda92593 (origin/main 7ab9a5fd plus this ticket). Old branch `fix/init-quota-off` stays as the cherry-pick source, read-only.
- Open fact for M2's plan turn: does main default `rules.quota` to off when the `rules` block is absent (c4744476, 92cfb7df)? Keep main's behaviour. Port the a7d2c765 default only if main has none, and say so in the plan.

- Plan turn (dev-1298, 2026-11-25) answered the open fact: main has no `rules.quota` default; a repo.yaml without `rules` fails schema validation (`spec/schemas/repo.schema.json:5,29`, `internal/claude/repo_config.go:68`). Host decision: port a7d2c765's default (`rules` optional, `quota: false`), proven by `TestParseRepoConfig_RulesOptional`.
- The M1 hunk of `runner_registry_test.go` holds the store-error assertion, not spool-write ones. Port the 571b0b69 error wording (`--store-dir`, `HARNEZ_STORE_DIR`) into `NewSessionStore` and its test into `store_dir_test.go`.
- `quota_text.go` keeps `managedTextFor`, `quotaTestCommand` and `plainTestCommand`. Drop its `rulesHeaderFor` and `quotaReadItem`, which duplicate `init.go:499`.
- The spool default `~/.harnez/spool` stays a Go default, matching main's `DefaultStoreDir`. Moving both paths to `spec/` is out of scope here.
- M3 doc: `docs/SessionIsolation.md` (root, not copyable), plus `docs/CLIDesign.md` and `docs/README.md` if needed.
- Picked commits keep their subject with the `-x` trailer. Hand-written commits carry `(issue 1298 M<n>)`.

## 5. Milestones

- **M1 (spool transplant):** cherry-pick 4a777882 (drop the deleted `internal/subagent/runner_registry_test.go`; move its spool-write assertions into `internal/subagent/spool_test.go` or a new file), then e465a670, 8eda0fe4, ec07b59f, 52b3951e. Use `git cherry-pick -x`. Do not cherry-pick any other branch commit.
- **M2 (quota-off init remainder):** port the parts of a7d2c765/ad261e05 that c4744476 does not cover: `internal/claude/quota_text.go` and its wiring, `internal/claude/quota_off_init_test.go` (scan every managed doc and rule file for `test-q1`), the repo schema changes if still needed, and the Spec wording in `docs/other/Spec.md` and its root copy `docs/Spec.md`. Then `make install`, `harnez init -d .`, `git checkout --` unrelated rewrites, and commit the source and the copy together.
- **M3 (spool docs):** document `HARNEZ_SPOOL_DIR`, its default `~/.harnez/spool`, and the `agent list` SOURCE values and `agent wait` rules in `docs/CLIDesign.md` or the session-isolation doc that main has. If the default path should be configurable, follow `docs/Spec.md`.
- **Verify:** `make install`, `make lint`, then one `HTO=0 make test-q1` after M3. Record its final summary line here.
