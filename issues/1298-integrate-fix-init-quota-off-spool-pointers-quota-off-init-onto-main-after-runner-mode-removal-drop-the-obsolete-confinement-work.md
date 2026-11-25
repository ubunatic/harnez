# 1298 — Integrate fix/init-quota-off (spool pointers, quota-off init) onto main after runner-mode removal; drop the obsolete confinement work

**Status**: Closed — Integrated via PR 226: spool pointers, list/wait/config --access, store-error wording and quota-off init on main; obsolete confinement work dropped; final full q1 80 ok, only failure is pre-existing 1299
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

## 6. Delivery and pre-work

**M1-M3 delivered** (dev-1298, 71fb1c32..20adf012): 5 spool and access picks, the store-error wording and test, the spool dir tests, the quota-off init port with quota-on and re-init tests, and the spool section in `docs/SessionIsolation.md`. `make install` and `make lint` pass, and `harnez init -d .` leaves the tree clean. The q1 run (`.git/harnez/quota-1-logs/test-20261125T225945.992926631Z.log`) failed in 3 tests. One of them, `TestRunLocalSelfInstallSymlinksGoBinToLocalBin`, is in a package this branch did not touch, so it went to #1299 and is not this sprint's job.

**M4 (q1 fixes), pre-work:**
1. `TestRunInit_QuotaOffManagedTextHasNoQuotaCommands` (`internal/claude/quota_off_init_test.go:56`) reads a Makefile that init never writes in that repo. Seed a Makefile with a plain `test:` target before init, and keep the no-`test-q1` assertion on the managed Makefile text.
2. `TestAgentListDefaultsToActiveAndHasDeadArchivedFilters` (`cmd/harnez/agent_list_test.go:143`) has two causes:
   - (a) Test isolation: cmd/harnez tests share one default spool, so registry writes from other tests leak in as `spool-stale` rows. Give each test its own `HARNEZ_SPOOL_DIR`, either in the shared env helper (`cmd/harnez/main_env_test.go`) or in `TestMain`. Do not weaken the list assertion.
   - (b) Product bug: the `--dead` list shows spool rows whose STATUS is `running` (`w00`-`w07`). Spool rows bypass the status filters. Run them through the same default, `--dead` and `--archived` filters as registry rows, and add a test with spool rows of each status for each filter.
3. Then rerun only the touched packages through the quota wrapper (`harnez exec --quota-1 -- go test ./cmd/harnez/ ./internal/claude/ ./internal/subagent/`), quote the summary line, and report that the full run is owed to the host.

**M4 delivered** (b459c9e7, adc24cdb, 3332cc87): the Makefile seed, the per-test spool in two list tests, and a filter-matrix guard test. My diagnosis in pre-work 2b was wrong: `--dead` means "include non-active" (`cmd/harnez/agent_manage_cmds.go:152`), so the spool rows already went through the filters correctly. The host's full q1 run (`.git/harnez/quota-1-logs/test-20261125T230840.259407721Z.log`): 80 packages ok, and the only failure is `TestRunLocalSelfInstallSymlinksGoBinToLocalBin`, which is #1299.

Review (rev-1298, opus): no HIGH findings. Removed code does not come back, and `agent wait` opens a spool store read-only and never saves.

**M5 (review fixes), pre-work:**
1. MED: `cmd/harnez/agent_async.go:510`. After the single trust check, `store.Find`/`Get` (`internal/subagent/session.go:463`) use plain `os.ReadFile`, so a confined writer can swap `<store>/<id>.json` for a symlink or a huge file. For a store opened from a spool entry, read entries without following symlinks and with the same 1 MiB cap as pointers. Re-verify the entry's session id on each poll.
2. LOW, fixed with 1: `internal/subagent/spool_read.go:164,205,226`. Lstat-then-Open races, and a FIFO hangs `agent list`. Open with `O_NOFOLLOW|O_NONBLOCK`, then `Fstat` and require a regular file.
3. MED tests: `spool_read_test.go:176` must assert that the returned store is read-only. Add forged and mismatched-id cases. Add one cmd-level test of `agent wait` on a verified spool entry and one that refuses a stale or unverified entry.
4. LOW: make spool isolation the default. In `cmd/harnez/main_env_test.go`, set `HARNEZ_SPOOL_DIR` per test through the shared env helper rather than opt-in `isolateSpool`, so tests like `agent_delete_test.go:172` and `agent_resume_test.go:84` cannot depend on order. If a per-test default is not possible with the current helper, use a package-level temp dir set in `TestMain` and keep `isolateSpool` for the list tests. Say which you did.
5. LOW: `internal/subagent/spool.go:30`. Trim `HARNEZ_SPOOL_DIR` as `DefaultStoreDir` does (`session.go:325`), and test a blank value.
6. LOW tests: `quota_off_init_test.go:37` also asserts that AGENTS.md has no `test-q1`. The quota-on test (`:87`) asserts that `make test-q1` survives in the managed text.
7. Docs: `docs/SessionIsolation.md` says only the last path part is checked for symlinks. Record the known limit: pointers are keyed by session id, so two stores that reuse an id share one pointer. Not fixed here.
8. Accepted, no change: the `rules.quota: false` spec default is read by `config get`, while `ParseRepoConfig` uses Go's zero value. The result is the same.
9. Verify: rerun the touched packages through the quota wrapper. The host runs the final full suite.

**M5 delivered** (88ef5fee..4141e7f8): spool-backed stores read through `readBounded` (no symlink, no FIFO, 1 MiB cap) and recheck the id on every `Get`. `Find` no longer falls back to a list. Pointer reads open, then Fstat. Tests cover `agent wait` on verified and refused entries. `HARNEZ_SPOOL_DIR` is trimmed. The known limits are in the doc. The host's final full q1 run after `make install`: 80 packages ok, and the only failure is `TestRunLocalSelfInstallSymlinksGoBinToLocalBin`, which is #1299.
