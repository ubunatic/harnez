# 1310 — Collector web server: interactive usage dashboards per session and time range

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: #1309 (claude_config_dirs, per-call pricing), #1311 (taia source plugin), #720 (datavis split, see Decision), #687 (TUI dashboard), #204 (sanitized export)

---

## 1. Problem & Motivation

To understand one session (example: d957448b, Haiku 5.5, $2.47, context peak 413k, two
compactions, 109 of 168 calls above the 100k price tier) we read report tables and recompute by
hand. The owner wants one interactive board: cost, context, prompt size and in/out/cache tokens on
a shared time axis, with filters per session and more.

taia's `usage-export` board was assessed (2026-11-27) and has the wrong granularity (one point
per turn, not per API call) and no prices for Claude Code sessions. harnez already has per-call
data, correct prices (Haiku 100k tier, `spec/pricing.yaml`), both Claude dirs via
`usage.claude_config_dirs`, and the quota timelines. The collector (`harnez server`) runs
permanently, so it can serve the board.

## 2. Technical Specification / Findings

- Collector today: HTTP over a unix socket, mode 0600 (`internal/server/server.go:80`), routes
  `GET /status`, `POST /shutdown`. A browser needs a TCP listener.
- Data: `internal/usagereport` (per-call usage, pricing, dedupe across config dirs) and the quota
  timelines used by `harnez usage report`.
- Board content (owner request):
  - per session and for a selected time range: cost (per call and cumulative), context (with the
    100k tier line and compaction markers), prompt size (input + cache read + cache write),
    input / output / cache read / cache write;
  - stacked charts on one shared time axis, draggable range, quota (weekly %, 5h %) aligned below;
  - filters: session, source (config dir), model, repo, provider; search; sort.
- Rules: static, no CDN, assets embedded in the binary (`docs/Website.md` spirit), vanilla JS/SVG.
  Read-only. Never serve prompt text, `compact_summary` or `custom_instructions`; numbers and ids only.
- New values (bind address, port, enable flag, refresh interval) go in `spec/` with a schema
  (`docs/Spec.md`), not as Go defaults.

## Decision

- 2026-11-27, owner: start a web server in the collector for interactive dashboards. This
  supersedes #720's "datavis leaves harnez" for usage dashboards; #720 keeps the rest.
- Open (security, owner confirms before M1 writes): TCP listener off by default, bound to
  127.0.0.1 only, with a per-start random token in the URL (or Unix-socket-only plus a
  `harnez usage web` local proxy). Proposed default: 127.0.0.1 + token. Unverified alternative.

## 3. Implementation & Verification Plan

/goal The collector serves a local interactive usage board with per-call charts and filters,
verified live against d957448b ($2.47, 168 calls, two compactions); or stop and report when
blocked on the listener security decision, a denied permission, or no progress (circuit breaker:
clear the goal and write a failure report here).

Check live code first; this ticket may be stale.

- **M1 (data API):** `GET /api/usage/sessions` (list + filters) and
  `GET /api/usage/sessions/<id>/calls` (per-call series: t, model, input, output, cache read,
  cache write 5m/1h, prompt size, cost, cumulative cost, context, compaction marks) from
  usagereport. Tests on fake transcripts in t.TempDir; golden JSON; the sum of call costs equals
  the report's session cost; no text fields in the payload (test).
- **M2 (listener + board shell):** spec-driven TCP listener per the Decision; embedded board
  page with session table and filters; one chart (cost). Tests: off by default, 127.0.0.1 bind,
  token required.
- **M3 (stacked charts):** context, prompt size, token types on a shared axis; draggable range
  that filters all charts and the table.
- **M4 (quota):** weekly and 5h quota lines aligned under the charts; range covers several sessions.
- Acceptance: host opens the board in a real browser (light and dark scheme, phone width) and
  checks d957448b against the report numbers above.
