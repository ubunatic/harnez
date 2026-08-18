# 031 — Live quota fetch failures are silent in `harnez usage`

**Status**: Closed — implemented 2026-08-18
**Category**: Bug — poor error visibility
**Discovered**: 2026-08-18, while adding `harnez usage --summary`

---

## Problem

`CollectClaude` (`internal/usage/claude.go:159-224`) makes a single live HTTP
call to `api.anthropic.com/api/oauth/usage` to populate `usage.Session`
(5-hour) and `usage.Weekly` (7-day) quota windows. When that call fails —
transport error, non-2xx status, or a JSON decode error — the failure is
either dropped entirely or stuffed into `usage.Details["live_quota_status"]`,
a map key nothing ever reads:

- `client.Do(req)` returning `err != nil` is not handled at all (`claude.go:169`)
- a non-200 response only sets `Details["live_quota_status"]`
  (`claude.go:220-222`), which is never rendered by `RenderText`
  (`internal/usage/usage.go`) or by the `--watch`/`--summary` grid
  (`internal/usage/watch.go`)
- a JSON decode error is silently ignored (`claude.go:173`)

The user-visible effect: `Session`/`Weekly` are just `nil`, and the panel
renders with fewer lines than expected — indistinguishable from "these
windows don't apply to this plan." Observed in practice: a `--watch` session
where Session/Weekly were missing on the first frame (an apparent transient
failure on that request) and appeared correctly on the next poll a minute
later, with nothing in the output indicating a fetch had failed.

Same story likely applies to `internal/usage/agy.go` and
`internal/usage/codex.go`'s live RPC/HTTP calls — worth checking whether they
have the same silent-failure shape.

## Proposed fix

- Track the live-fetch outcome on `AgentUsage` explicitly (e.g. a
  `QuotaFetchError string` field) whenever the HTTP call errors, returns
  non-2xx, or fails to decode.
- Render it: in the `--watch`/`--summary` panel, a line like
  `quota: unavailable (timeout)` when Session/Weekly are absent *because of*
  a fetch error, distinct from an agent that genuinely has no quota windows.
- In `RenderText`, surface the same via `Details["live_quota_status"]` (which
  already exists but is never printed).

## Related

- [Issue 032: watch shouldn't blank stale-but-good quota data](032-usage-watch-no-stale-fallback-on-fetch-failure.md) — same root cause, complementary fix
- [Issue 023: `harnez usage`](023-usage-command-token-quota-tracking.md) — original command
