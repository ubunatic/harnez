# 714 — harnez usage shows Claude/GPT and Gemini unavailable after collectors are killed

**Status**: Open — live repro 2026-10-22: stale threshold equals the 5m collector interval
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 710, [[527-agy-live-quota-fetch-is-killed-on-most-agent-turn-boundaries]],
[[649-harnez-usage-compact-no-longer-shows-agy-gemini-quota-rows]],
[[657-usage-compact-drops-claude-and-codex-writers-store-label-window-keys-cross-provider-observation-links-codex-limits-not-collected]]

---

/goal Restore current Claude/GPT and Gemini usage rows in `harnez usage`, with regression
coverage and live verification, or stop and report when blocked on user input or denied permission.

## 1. Problem & Motivation

User report: the `harnez usage` UI shows both AGY quota pools as unavailable:

```text
Claude/GPT unavailable (signal: killed)
Gemini unavailable (signal: killed)
```

Both usage rows are affected. The signal does not identify which process was
killed or whether a shared collector, timeout, or cancellation caused it. The
host, exact command, and observation time were not provided; collect them during
reproduction.

## 2. Findings

Related issues cover earlier AGY fetch kills (#527), missing Gemini rows (#649),
and usage-store projection failures (#657). This is a fresh observation; reproduce
it against current code and determine whether it shares one of those causes before
choosing a fix.

## 3. Implementation & Verification Plan

- Trace the failing collection path and identify who sends the kill signal and why.
- Fix the cause or expose a useful, provider-specific failure while retaining any
  valid cached usage.
- Add regression coverage for both pools; run `make test-q1` and verify the
  installed `harnez usage` UI when the required live services are available.

## 4. Recurrence (2026-10-22)

User saw it again in the live `harnez usage` view, now on three rows:

```text
Claude/GPT unavailable (signal: kill…
Gemini 4 Argon unavailable (signal: kill…
Gemini unavailable (signal: kill…
```

Since the original report, collection moved to `harnez server` with a 5-minute snapshot
cadence (828-830, 837, 839), so the killer may now be the server collector's timeout or socket
takeover (837) rather than a client. Check that path first. Rows also go gray often in the same
view; see 710.

## Live check (2026-10-22, lean sprint): not reproducible today

`harnez usage --raw` showed fresh Claude, Codex and both AGY pools. Gray comes from either a `QuotaFetchError`
(`internal/usage/types.go:160`) or snapshot age (`internal/usage/freshness.go:5`); the display does not say
which. Way forward: show the reason on a gray row (fetch error text or age), keep the last good quota on a
failed refresh, test fresh/stale/failed rendering. Then park until the next live occurrence, which the reason
text will explain.

## Live repro (2026-10-22 15:30, v0.2.0, user screenshot of `harnez usage -w`)

Claude Code and all AGY rows gray, Codex white. `harnez usage` at 15:30:34: Claude "5m ago", AGY "9m ago",
Codex "4m ago". No fetch errors: `--json` has no error field, AGY refreshed fine at 15:33:38.

Root cause (gray without a failure): `internal/usage/freshness.go` marks a snapshot stale at
`age >= DefaultCollectorInterval()` (5m), the same 5m cadence the server collects at. Every provider goes gray
for a moment before each refresh, and longer when a cycle runs late or is skipped. AGY at 9m means one missed
cycle: the server was restarted at 15:25:22 (v0.2.0 install) and the first cycle after the restart did not
refresh AGY.

Also seen: the gray row in `-w` showed no reason (the 714 reason text didn't appear in the cropped view;
`--json` carries no staleness field), and every cycle logs
`drain telemetry spool: ... CHECK constraint failed: score ... BETWEEN 1 AND 5 (275)`, so one bad spool
entry fails each drain.

Way forward: stale only after a grace beyond the cadence (e.g. 2 intervals, value in `spec/usage.yaml`);
collect all providers in the first cycle after a server start; expose `stale`/`reason` in `--json`;
skip or quarantine the invalid spool entry instead of failing the drain. Test that a snapshot exactly one
interval old renders normal, not gray.

Statusline feed (checked 15:45): both statuslines are wired (`harnez statusline`, `harnez statusline --agent agy`
in `~/.gemini/antigravity-cli/settings.json`) and post to `/usage/passive`. Live: one Claude statusline render
moved the Claude row from "1m ago" to "just now". Yet AGY stayed 9m old with an AGY session open, so either the
AGY payload carries no quota fields (`passiveWindows` then drops it silently) or AGY renders only on activity.
Check: log or count dropped statusline observations per provider; capture one real AGY payload.

## Decision (user, 2026-10-22): demand-driven catch-up fetch

The fixes above are approved, with this amendment to "collect all providers in the first cycle after a start":

- On server start with no consumer connected, skip the extra initial fetch; keep only the regular 5m cadence.
- When a consumer connects (usually `harnez usage -w`), run one extra fetch at once for every provider whose
  last successful fetch is older than 1 minute (value in `spec/usage.yaml`); providers fetched within that
  minute are left alone.
- Open design point: how the server learns a consumer connected. Today the UI reads the store files; it would
  need to announce itself on the socket (e.g. a request on connect), which the server treats as the trigger.

## Decision (user, 2026-10-22, second amendment)

- **Client heartbeat:** consumers (`harnez usage -w` and other viewers) send a periodic heartbeat to the server
  over the socket, although they read the data from the store directly. The server thereby knows how many
  clients are watching; the first heartbeat after none triggers the catch-up fetch above. Interval and
  client timeout in `spec/usage.yaml`; `harnez server status` (and `--json`) shows the client count.
- **Staleness reason in the UI, not only JSON:** the `!` key, which today shows the per-provider fetch
  spinner, also opens a debug box below it with each provider's last fetch time, stale reason and error
  message.
- **General debug box:** the usage UI gets one debug box that collects these messages, toggled by its own key
  (a free letter or digit; pick one that no view uses and list it in the help line).

## Recurrence (2026-10-22 15:50, v0.2.0): "signal: killed" on all three AGY rows

`harnez usage --json` at 15:50: AGY `quota_fetch_error: "signal: killed"`, `last_refreshed`
13:42:37Z (15:42 local); the last good quota (6m old) is still shown, so the cached values survive.
`harnez server logs -n 300` has no line about the kill, timeout or AGY at all, while the same window holds
hundreds of identical `telemetry: no tool call found for session ...` lines. So the fetch error is stored
but never logged. Way forward, in addition to the above: log every fetch failure with provider, command,
elapsed time against its timeout, and who sent the signal (our context timeout vs. external); show it in
the debug box; rate-limit the repeated telemetry line. Then find whether the AGY fetch hits its timeout.

## Sprint (2026-10-22, dev-714 flash38:low)

- M1 delivered `d7a372c2` (stale grace): reuses `cache_staleness_multiplier` (now 2) for display staleness;
  `stale`/`stale_reason` in `--json`; host reran usage and cmd tests (green) and saw `"stale": false` for all 3
  agents live. Note: the same key also widens the client cache freshness window to 2 intervals (intended).
- M2 delivered `b6ed79ba` (fetch-failure log, telemetry warning rate limit `telemetry_warning_rate_limit: 1m`,
  invalid spool entries moved to `quarantine/`); host reran `./internal/... ./cmd/harnez` tests: green.

### Pre-Work for M3 (from M2 review)

1. Cause is misclassified: `exec.CommandContext` kills the child with SIGKILL when *our* context deadline
   passes, so "signal: killed" is usually our own timeout, not an external signal. Decide the cause from the
   context (`ctx.Err()` is `DeadlineExceeded`, or elapsed >= timeout), not from the error text; only a kill
   with a live context counts as external. Test both cases.
2. Prove the log fires for the real failure path: a collector that returns an `AgentUsage` with
   `QuotaFetchError` set (and a nil error), as AGY did at 15:42, must also produce the log line. Test it.
3. The `Command` strings in `registry.go` duplicate endpoint URLs and the agy command; take them from where the
   fetchers already define them (or spec), not new literals.
- M2 follow-up `2432aa10` (cause from context, reused endpoints), M3 `9e247bf8` + `829a4af7` (heartbeat,
  `clients:` in server status, catch-up). Host check 2026-10-23: tests green; a pty viewer showed
  `clients: 1` for 20s, `0` after 40s; all three providers refreshed within 4s of the viewer start.

### Pre-Work for M4 (from M3 review)

1. Log one line when a catch-up fetch fires (client count 0→1, which providers, which were skipped as
   fresh); today it is silent, so the live check can only infer it from `last_refreshed`.
- M4 handover (2026-10-23 13:30): dev-714 (agy flash) stalled after 467 steps and was stopped with
  uncommitted, untested M4 work; dev-714b (sol:med) took over the tree. dev-714's own status report (13:12):
  catch-up log line `harnez server: catch-up fetch triggered: clients 0->1, fetching: <providers>, fresh
  (skipped): <providers>`; `!` toggles the countdown overlay and a debug box below "All Usage" (last_refreshed,
  stale_reason, fetch error/cause); general toggle `toggle_debug` on key `d`/`D` (category box), shown as
  `[d]ebug` in the footer and the `?` reference. Its `make test-q1` run never reported.
- Next: M2 (fetch-failure logging, telemetry line rate-limit, bad spool entry skipped), M3 (heartbeat +
  catch-up, keys in `spec/usage.yaml`), M4 (debug box), M5 (AGY statusline drop count).
