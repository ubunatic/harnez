# Case Study: Quota-Fetch Robustness, `--summary`, and a Flock-Coordinated Shared Cache

**Date**: 2026-08-18
**Scope**: `harnez usage --summary`, three robustness fixes to live quota fetching in `internal/usage/`
**Feature Issue**: [Issue 023: `harnez usage`](../../issues/023-usage-command-token-quota-tracking.md), continued
**Related Issues**: [031](../../issues/031-usage-quota-fetch-errors-silent.md), [032](../../issues/032-usage-watch-no-stale-fallback-on-fetch-failure.md), [033](../../issues/033-usage-shared-quota-cache.md)
**Status**: Implemented, committed (`6c82176` code, `fc74910` website docs)

---

## 1. Context

Follow-on session to [2026-08-17's multi-agent usage monitor](2026-08-17-multi-agent-quota-and-usage-monitoring.md)
and [2026-08-18's `--watch` TUI build](2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md).
Started as a small ask — add a one-shot `--summary` flag that prints the compact `--watch` grid
without entering the live loop — and turned into a chain of real bugs discovered by using the
feature normally, not by code review.

## 2. What happened, in order

1. Built `--summary` (`internal/usage/watch.go`: `RenderSummary`, plus a `live bool` parameter
   threaded through `buildWatchFrame`/`buildAgentBox` so the static frame omits the refresh-rate
   footer and the tokens/min sparkline that only make sense with a running interval).
2. User reported that Claude's Session/Weekly quota windows were missing when `--watch` first
   started, then appeared correctly a minute later on the next poll. Read `CollectClaude`
   (`internal/usage/claude.go`) to explain it: a single unguarded HTTP call to
   `api.anthropic.com/api/oauth/usage`, with every failure mode (transport error, non-2xx, decode
   error) either silently ignored or written to `Details["live_quota_status"]`, a map key nothing
   ever rendered. That explanation became [Issue 031](../../issues/031-usage-quota-fetch-errors-silent.md).
3. Also flagged the complementary gap: `--watch` had a full previous frame in memory
   (`lastSummary`) and threw it away on a failed refresh instead of falling back to it. That became
   [Issue 032](../../issues/032-usage-watch-no-stale-fallback-on-fetch-failure.md).
4. Both issues were written up and handed to a background subagent (`general-purpose`,
   non-interactive) to implement. It added `AgentUsage.QuotaFetchError`, rendered it in both
   `RenderText` and the compact grid, and added `applyStaleQuota`/`staleLabel` to `RunWatch` to
   carry forward stale-but-good windows.
5. While verifying `--summary` against a running `--watch`, the user checked running processes and
   found **two `harnez usage --watch` instances** running concurrently — one a leftover `go run`
   from earlier testing (30s interval), one the real session (60s interval) — both independently
   polling the same account's quota endpoint. Shortly after, a real `HTTP 429` appeared. That was
   the direct trigger for [Issue 033](../../issues/033-usage-shared-quota-cache.md): a shared,
   cross-process cache.

## 3. The design conversation before the issue (worth preserving)

Issue 033 wasn't written straight from a one-line ask — it went through several rounds of
back-and-forth that changed the actual mechanism, and the reasoning is what has lasting value, not
just the final code:

1. **First framing**: "save the last successful API response next to the other Claude cache files,
   and detect on startup whether another instance recently wrote it." This is a peer-detection
   framing — explicit "is someone else running" logic.
2. **Reframed as TTL cache-aside**: the peer-detection question turns out to be unnecessary. A
   process doesn't need to know *who* wrote the file or *why* it's fresh — it only needs to know
   "is it fresh enough that I don't need to fetch." That's a strictly simpler rule, symmetric across
   every process (including a lone one), and self-healing (a crashed writer just leaves a file that
   ages out on its own, no peer state to reconcile).
3. **The write race**: cache-aside answers "should I fetch," not "is it safe to write what I
   fetched." User pushed on this directly — "we need to check that someone else doesn't have just
   written it" — surfacing a real write-write race: two processes can both decide to fetch (cache
   was stale for both), both succeed, and now both are about to write.
4. **First attempt at a fix: three-step optimistic check** (check freshness before fetching,
   re-check-before-write to avoid clobbering something fresher, read-back after writing to detect
   being clobbered anyway). Worked through in detail, including *why* a post-write check is still
   needed even with a before-write check — the check and the write aren't one atomic operation, so
   there's a TOCTOU gap a timestamp comparison alone can't close, and mtime granularity can be too
   coarse to tell two near-simultaneous writers apart.
5. **Asked directly: is this a standard pattern?** It isn't, cleanly — write of a whole file via
   temp+rename is a standard atomic-replace idiom, but layering manual timestamp comparisons around
   it to *simulate* compare-and-swap is not a named technique; POSIX files don't offer real CAS, so
   the three-step protocol is a heuristic approximation of optimistic concurrency control, not the
   real thing, and it ends up being *more* code than the actual standard tool for this exact
   job: `flock`.
6. **Landed on `flock`**: exclusive, advisory, held only around "fetch + write," released
   automatically by the kernel on any process exit (crash, panic, SIGKILL — no PID-file or
   stale-lock bookkeeping needed), with a bounded non-blocking retry so a lock holder that hangs
   can't wedge every other process forever.

The three-step protocol was good enough to *build*, but wrong to *ship* — the design conversation
caught that before any code existed, which is the actual point of doing the discussion turn by turn
in chat instead of jumping straight to an issue file. **Issue 033 explicitly records the rejected
approach and why**, specifically so a future implementer (human or agent) doesn't rediscover and
re-adopt it from a plausible-looking first-principles read of the problem.

## 4. Final mechanism (implemented)

- `~/.claude/harnez-quota-cache.json` — `fetched_at`, `session`, `weekly`.
- **Read**: before any live call, check the cache's age against `MinWatchInterval` (30s, already
  existed as the `--watch` interval floor — reused rather than inventing a second constant). Fresh
  → use it, skip the network entirely. This is what actually kills the double-polling: after any
  one process refreshes, every other process's next tick sees a warm cache.
- **Write**: only reached when the cache was stale/missing. `syscall.Flock(LOCK_EX|LOCK_NB)` on a
  `<cache>.lock` sidecar, 5 retries at 50ms (~250ms bounded wait total). Lock acquired → fetch, then
  atomic write (`.tmp` + `rename`), then unlock. Lock not acquired in time → still fetch live and
  use the result for this call, just skip the disk write (a sibling is presumably writing its own
  fresh copy right now).
- **On fetch failure**: fall back to the disk cache regardless of age, labeled `(stale)` — the disk
  version of issue 032's in-memory fallback, and it also covers one-shot `--summary`/`usage` calls
  that have no in-memory `lastSummary` to fall back to at all.

No new dependency: `golang.org/x/sys` wasn't in `go.mod`, and `watch.go` already called `syscall`
directly for `SIGTERM`/`SIGWINCH`, so `syscall.Flock` matched the existing style.

## 5. Agentic-process learnings

1. **A live bug found by dogfooding beat any amount of code review.** Nobody would have flagged the
   silent-failure or double-polling issues by reading `claude.go` cold — they surfaced because the
   user actually ran `--watch` twice by accident and actually hit a 429. This argues for treating
   "use the feature you just built, for real, across a normal session" as part of verification, not
   an optional extra — consistent with [`docs/studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md`](2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md)'s
   conclusion that some bug classes only show up under real conditions a static read can't simulate.
2. **Design-by-conversation before the issue file is worth the turns it costs.** The three-step
   protocol looked reasonable in isolation and would have been the shipped design if the issue had
   been written after step 3 above instead of after step 6. The "is this standard?" question in
   particular was the one that actually dislodged it — naming the pattern space (CAS, OCC, flock,
   Maildir-style atomic rename) surfaced the better-known, lower-code answer.
3. **Recording a rejected approach inside the issue file, not just the shipped one, is cheap
   insurance.** A subagent (or a future human) reading only "use flock" without the "here's what we
   tried first and why it's worse" context could plausibly reinvent the three-step version from
   scratch — it's not an unreasonable design, just a worse one for this specific job.
4. **Background subagents worked cleanly for both rounds** (031+032 together, then 033 separately)
   — each got a self-contained prompt with concrete file/line references and the actual design
   decision already made, not "figure out the best approach." Delegation was for *implementation
   labor*, not *design judgment*; the design judgment happened in conversation first. This matches
   the "never delegate understanding" principle — the prompts encoded decisions, not open questions.
5. **Stray processes are an easy blind spot in agent-run sessions.** The extra `harnez usage --watch`
   that contributed to the 429 was a `go run` process from earlier testing in *this* conversation,
   left running silently. Worth remembering to check `ps aux` for leftover long-running processes
   (`--watch`, dev servers, etc.) started earlier in a session, especially before attributing an
   external symptom (a 429) purely to "the code has a bug" rather than "the code has a bug *and*
   there's stale local state contributing to it."

## 6. Related

- [Issue 023](../../issues/023-usage-command-token-quota-tracking.md) — original `harnez usage` command
- [Issues 031, 032, 033](../../issues/) — the three fixes from this session
- [2026-08-17 multi-agent usage monitoring case study](2026-08-17-multi-agent-quota-and-usage-monitoring.md) — corrected in place; its "gracefully falls back on 429" claim was inaccurate until this session's fix
- [2026-08-18 `--watch` TUI postmortem](2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md) — prior session in the same feature area, same "real usage surfaces real bugs" lesson
