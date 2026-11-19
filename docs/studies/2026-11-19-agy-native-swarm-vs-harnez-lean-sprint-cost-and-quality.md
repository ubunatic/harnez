---
kind: benchmark
title: "AGY Native /swarm vs. Harnez /lean-sprint: Cost, Quota, and Quality Comparison (#1192, #1236)"
---

# Benchmark Study: AGY Native `/swarm` vs. Harnez `/lean-sprint` (`#1192`, `#1236`)

**Date:** 2026-11-19  
**Branch:** `docs/story-and-website-2026-11-19`  
**Swarm Run:** `12:40:36 CET` – `12:52:17 CET` (`11m 41s` from first `add` to `@verifier [done]`; `~18m 32s` total session wall-clock from `12:34:28 CET` including `/pm` setup and user Q&A)  
**Tickets Completed:** [#1192](../../issues/1192-harnez-release-on-protected-main-goes-through-an-agit-pr-tags-after-the-merge.md) (`P1`, commit `8c99b779`, closed `c44d3c2d`), [#1236](../../issues/1236-telemetry-openreadonly-lacks-busy-timeout-causing-sqlite-busy-on-stats-agents.md) (`P2`, commit `962ad4fd`, closed `f0ca94e6`)

## 1. Context

- **Goal:** Run a real two-ticket maintenance batch (`#1192` in `internal/release/`, `#1236` in `internal/telemetry/`) using Antigravity's built-in `/swarm` skill with `gemini-4-hydrogen-medium` workers (`workspace: "inherit"`), capture live quota and transcript metrics, and compare **cost vs. quality** against Harnez's `/lean-sprint` workflow observed on the same codebase (`0eb37853..96ef7c74`).
- **Swarm Roster (`gemini-4-hydrogen-medium`, 4 workers + host coordinator):**
  - `release-1192` (`d91e3477-8d9c-4529-b02b-0b15c0a267b7`) — author for `#1192` (`push_pr.go`, `gitflow_test.go`).
  - `telemetry-1236` (`f8119b9b-52e2-4208-be7b-4e507e71fe6a`) — author for `#1236` (`telemetry.go`, `telemetry_test.go`).
  - `verifier` (`48cd5472-46ef-4775-a743-254b698c5aed`) — read-only diff reviewer and Quota-1 test/lint gatekeeper.
  - `monitor` (`78c9ea70-d621-4c91-850b-09f813434f7b`) — `swarm_monitor` stall watcher.
- **Pre-Swarm Quota Baseline (`12:34:41 CET`, `swarm models`):**
  - `Gemini 5h` pool (`gemini-4-hydrogen-*`, `gemini-4-neon-*`, `gemini-3.8-flash-*`): `39%` remaining (`61%` used; resets `14:45:42Z`).
  - `Gemini 4 Argon weekly` pool (`gemini-4-argon-*`, `Orchestrator (Argon)`): `90%` remaining (`10%` used; resets `2026-11-26T09:50:53Z`).
  - `Gemini weekly` pool (`harnez usage`): `42%` session baseline (`49%` at `12:47:08 CET`).

## 2. Outcome

### 2.1 Code & Quality Delivered

Both tickets shipped in one `11m 41s` swarm cycle with clean, minimal diffs (`4` files, `+213 / -7` lines) and zero zombie workers:

- **#1192 (`8c99b779`, `+104 / -6`):** `releasePRMergedAhead` and `pushReleaseViaPR` in `internal/release/push_pr.go` now detect squash- or rebase-merged protected-main release PRs via `releasePRSquashedOrRebased` and return an actionable error instead of re-opening the PR; `internal/release/gitflow_test.go` gained second-remote mirroring assertions in `TestReleaseOnProtectedMainGoesThroughPR`, plus `TestReleaseOnProtectedMainDetectsSquashMerge` and `TestReleaseOnProtectedMainRefusesTagOnMergeCommit`.
- **#1236 (`962ad4fd`, `+109 / -1`):** `OpenReadOnly` in `internal/telemetry/telemetry.go` now sets `_pragma=busy_timeout(5000)`; `TestOpenReadOnlyWaitsForBusyLock` in `internal/telemetry/telemetry_test.go` verifies `PRAGMA busy_timeout == 5000`, reproduces raw `?mode=ro` lock failure under `BEGIN EXCLUSIVE` (in `DELETE` journal mode), and proves `OpenReadOnly` + `Query(Filter{})` succeeds across a `40ms` write lock.
- **Gate checks (final edit verified by `verifier` `#17`, then `make install` by host):** `HTO=0 make test-q1` exit `0` (Pi `132/132`, stage install `PASS`, all Go suites `ok`); `HTO=0 make lint` exit `0` (`golangci-lint`: `0 issues.`, `scripts/lint.sh`: `lint: ok`).

### 2.2 Measured Cost & Transcript Activity

Data collected directly from `swarm models`, `./harnez usage`, `./harnez stats --agents --days 1`, and per-session `transcript.jsonl` files:

| Role / Session | Elapsed UTC Window | Steps | `PLANNER_RESPONSE` | Tool Calls | Board Posts | `harnez` Fitted 5h Drain |
|---|---|---|---|---|---|---|
| `release-1192` | `11:40:40Z..11:47:51Z` (`7m11s`) | 54 | 19 | 31 (`run_command: 16`, `edit_file: 10`, `post_message: 2`, `swarm_wait: 2`, `read_board: 1`) | 2 (`#7`, `#12`) | `3.0%` |
| `telemetry-1236` | `11:40:43Z..11:49:43Z` (`9m00s`) | 52 | 18 | 26 (`run_command: 8`, `edit_file: 6`, `read_board: 4`, `post_message: 4`, `swarm_wait: 4`) | 4 (`#6`, `#9`, `#13`, `#15`) | `3.0%` |
| `verifier` | `11:40:51Z..11:52:16Z` (`11m25s`) | 84 | 32 | 32 (`run_command: 13`, `read_board: 8`, `swarm_wait: 7`, `post_message: 4`) | 4 (`#8`, `#10`, `#14`, `#17`) | `3.8%` |
| `monitor` | `11:40:53Z..11:41:11Z` (`18s` active) | 15 | 5 | 8 (`read_file: 3`, `run_command: 1`, `read_board: 1`, `schedule: 1`, `post_message: 1`, `swarm_wait: 1`) | 1 (`#5`) | `1.6%` |
| **Workers subtotal (4)** | **`11m 41s` wall** | **205** | **74** | **97** | **11** | **`11.4%` sum** |
| Coordinator (`deaaef4c`, since `11:34:28Z`) | `11:34:28Z..11:52:41Z` | 114 | 41 | 56 (`run_command: 20`, `swarm: 17`, `read_board: 7`, `exit_plan_mode: 5`, `swarm_wait: 3`, `post_message: 3`, `enter_plan_mode: 1`) | 7 (`#1`–`#4`, `#11`, `#16`, `#18`) | shared / `—` |

- **Live AGY Quota Deltas (`12:34:41 CET` → `12:52:46 CET`, `harnez usage` at `0m ago`):**
  - **`Gemini 5h` (`gemini-4-hydrogen-medium` workers + coordinator execution):** `61% → 70.53%` (**`+9.53%` of a 5h window** for 2 tickets, ~`+4.77%` 5h per ticket; mid-swarm `swarm models` already showed `+7%` at `12:49:44 CET`).
  - **`Gemini 4 Argon weekly` (host `<plan_execute_protocol>` planning turns):** `10% → 23.40%` (**`+13.40%` of the 7-day Argon quota** across 5 planning exits before/during the swarm!). By `12:49:44 CET`, Argon weekly had already risen `+12%` (`90% → 78%` remaining).
  - **`Gemini weekly`:** `49%` (`12:47:08 CET`) → `49.74%` (`12:52:46 CET`, `+0.74%` in the final 5 minutes; session baseline earlier was `42%` before `/pm` triage + `/story`).
- **Telemetry Visibility in `harnez stats --agents`:** AGY `/swarm` sessions register as `source: host`, `provider: agy`, with blank `MODEL`, `TURNS`, `NEW INPUT`, `CACHED`, and `OUTPUT` (`—`) and `5H DRAIN` marked `fitted/shared`, plus `UNROUTED` shell counts (`8` on `telemetry-1236`, `13` on `verifier`).

### 2.3 Cost vs. Quality: AGY `/swarm` vs. Harnez `/lean-sprint`

Comparison uses observed sprint metrics from [the 2026-11-19 story](2026-11-19-session-isolation-forge-parity-and-quota-pack-story.md) and sprint studies ([#944](2026-11-19-two-agent-lean-sprint-944-quota1-gotest.md), [#771](2026-11-19-two-agent-lean-sprint-771-exec-truncation.md), [#1136](2026-11-19-two-agent-lean-sprint-1136-release-bump-infer.md), [#1243](2026-11-19-two-agent-lean-sprint-1243-neon-costs.md)):

| Dimension | AGY Native `/swarm` (`hydrogen:medium`, `#1192` + `#1236`) | Harnez `/lean-sprint` (observed `#1136`, `#944`, `#771`, `#1243`) |
|---|---|---|
| **Wall-clock latency (2 tickets)** | **`11m 41s` swarm time** (true parallel authoring + single shared `make test-q1` / `make lint`) | ~`20–26m` per ticket sequentially unless multiple host terminals/worktrees run (`#1136`: `~19m36s`, `#944`: `~26m`) |
| **Coordinator quota cost** | **Expensive (`+13.40%` weekly Argon)** when AGY `<plan_execute_protocol>` re-runs the planning model on board wakeups (`5` `exit_plan_mode` cycles) | **Low host overhead** (single prompt dispatch -> `harnez agent wait`; no host code digging, host review is short diff inspection) |
| **Worker quota / cost tiering** | Single provider (`agy`); defaults to `gemini-4-hydrogen-medium` (`+9.53%` 5h for both tickets) unless user requests otherwise; hot quota switching keeps conversation | **Mixes cheapest model per role** (`claude:haiku:low/med` scouts at `~43s–75s` and developers at ~250k–7M cached tokens, `claude:opus:med` / `agy:neon:med` cross-vendor reviewers) |
| **Verification & defect catching** | **High in-loop responsiveness:** `verifier` caught 2 `errcheck` lints (`#8`) and a real WAL-vs-`DELETE` test bug (`#14`, where raw `?mode=ro` did not fail under `BEGIN EXCLUSIVE` in WAL mode) via direct peer ping in seconds | **Highest independence:** second-vendor review (`claude:opus:med` vs `agy:neon`, etc.) caught subtle cross-vendor blind spots (`#771` state bug, `#1206` remote-blind guard) in `1m20s–2m47s` |
| **Harnez guardrail fit** | **Friction with `Quota-1` & `schedule`:** shared `workspace: "inherit"` causes workers' `go test -c` to consume `verifier`'s single Quota-1 run; `harnez` hook blocks `swarm_monitor`'s `schedule` cron | **Native fit:** enforces Quota-1 per developer/host turn, records full `NEW INPUT`/`CACHED`/`OUTPUT`/tier telemetry in `tool_catalog.sqlite`, integrates commit-by-milestone & ticket closure |

**Bottom line on Cost vs. Quality:**
- **Quality per bug fix** from `gemini-4-hydrogen-medium` in `/swarm` was strong: `release-1192` cleanly folded helpers on review feedback and `telemetry-1236` + `verifier` caught and proved the SQLite WAL snapshot bug (`#14`–`#15`). However, **single-vendor review misses cross-vendor diversity**, and all coordinator review quality depended on the host coordinator catching duplication/repro gaps in plan mode (`#11`).
- **Quota economics strongly favor `/lean-sprint` when coordinator planning is on AGY Argon weekly (`+13.40%` weekly Argon burn for two small-to-medium bugs in `/swarm`)** vs. saving `/swarm` for truly parallel, independent multi-file tasks where shared live peer iteration outweighs cross-vendor review and weekly Argon cost.

## 3. What Went Wrong

1. **Coordinator read Go files upfront instead of delegating to a scout:** Following AGY's default `<plan_execute_protocol>` habit, the coordinator read `internal/release/push_pr.go`, `internal/release/gitflow_test.go`, and `internal/telemetry/telemetry.go` before spawning the swarm instead of keeping Turn 1 purely to ticket-level partitioning (violating `/swarm` "Do not do the work"). Caught by user prompt (`12:41:01 CET`).
2. **`swarm_monitor` `schedule` blocked by Harnez hook (`#5`):** `monitor` attempted to register its 3-minute stall-check cron, was blocked by the Harnez `PreToolUse` hook (*"violates the harnez rule to not use schedules..."*), and parked on `swarm_wait`. Impact was minor because active board traffic kept moving, except between `#15` and `#16` when all workers sat `waiting` for `~1m20s` until nudged (`#16`).
3. **`go test -c` consumed the Quota-1 test slot (`#10`):** `telemetry-1236`'s compile-only `GOWORK=off go test -c -o /dev/null ./internal/telemetry` at `11:43:12Z` after fixing `errcheck` triggered `harnez exec`'s Quota-1 timestamp (`#944` detector), blocking `verifier`'s subsequent `HTO=0 make test-q1`. Fixed in-sprint by instructing authors in `#11` to use `GOWORK=off go vet` only alongside real review refinements that updated source files. Filed as **[#1246](../../issues/1246-exec-quota-1-treats-compile-only-go-test-c-as-a-test-execution-blocking-verifier-runs.md)**.
4. **WAL snapshot reads masked the `#1236` reproduction check (`#13` → `#14` → `#15`):** `telemetry-1236` initially held `BEGIN EXCLUSIVE` on a WAL-mode database (`Open(path)` enables WAL), where `?mode=ro` snapshot reads still succeed without `busy_timeout`. `verifier`'s first `make test-q1` caught it (`#14`), and `telemetry-1236` fixed `TestOpenReadOnlyWaitsForBusyLock` (`PRAGMA journal_mode = DELETE`, `#15`) before the final `make test-q1` pass (`#17`).
5. **Silent park between `#15` and `#16`:** After `telemetry-1236` posted `@verifier [done]` (`#15`, `12:49:44`), `verifier`'s status showed `waiting` at `last_read_seq: 15` with no reply until the coordinator nudged `@verifier` in `#16` (`12:51:07`). Likely `verifier` woke at the tail of its previous turn or skipped re-running without a post.

## 4. Learnings

- **Disable or minimize planner round-trips during `/swarm` coordination, and never pre-read code as coordinator:** Re-entering `<plan_execute_protocol>` on board wakes burned **`+13.40%` of weekly Argon quota** for two bugs, dwarfs the **`+9.53%` 5h Hydrogen** burn of all 4 workers combined, and delays dispatch.
- **In shared-workspace `/swarm` runs on Harnez repos, ban `go test` *including `go test -c`* from authors until [#1246](../../issues/1246-exec-quota-1-treats-compile-only-go-test-c-as-a-test-execution-blocking-verifier-runs.md) ships:** Authors should validate solely with `GOWORK=off go vet ./...` and `go build`, leaving `HTO=0 make test-q1` exclusively to `@verifier`.
- **Keep `/lean-sprint` as the default for 1–2 ticket bugs and serial features; use `/swarm` only for genuine `4+` independent parallel units** (or when AGY's local quota pool has plenty of headroom and cross-agent peer debugging on a shared board saves multiple turnaround cycles).
- **For SQLite busy-timeout tests, never rely on `BEGIN EXCLUSIVE` in WAL mode to block readers:** Switch the test handle to `PRAGMA journal_mode = DELETE` (or verify against two writers / `PRAGMA locking_mode = EXCLUSIVE`) and prove the failure *without* `busy_timeout` inside the same test.

## 5. Follow-Up Tickets

- **Allocated with this study (Ticket Extraction Gate):**
  - **[#1246](../../issues/1246-exec-quota-1-treats-compile-only-go-test-c-as-a-test-execution-blocking-verifier-runs.md)** (`Open`, `P2`, Bug) — Exempt compile-only `go test -c` invocations from Quota-1 test-execution counting in `harnez exec` so authors can compile test binaries without burning `@verifier`'s or the host's single `make test-q1` run.
- **Pre-existing / External Scope Recorded as Justified Non-Goals:**
  - *Allowing `schedule` in `swarm_monitor`:* **Non-goal** — Harnez deliberately disables `schedule` in favor of background tasks and reactive wakeups; `swarm_monitor`'s `swarm_wait` fallback already parks at zero token cost (`1.6%` total 5h drain).
  - *Changing AGY native `/swarm` token accounting (`MODEL`/`NEW INPUT` blanks in `harnez stats --agents`):* Deferred until AGY exports per-conversation token usage without transcript crawling (tracked broadly under [#034](../../issues/034-hook-triggered-token-extraction.md) / [#691](../../issues/691-track-claude-and-agy-sessions-in-agent-list.md) in Backlog).
