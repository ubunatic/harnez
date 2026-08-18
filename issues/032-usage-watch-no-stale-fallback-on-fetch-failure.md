# 032 — `--watch` blanks quota panels instead of keeping last-known-good data on a transient fetch failure

**Status**: Closed — implemented 2026-08-18
**Category**: Bug — robustness
**Discovered**: 2026-08-18, while adding `harnez usage --summary`

---

## Problem

Every tick of `harnez usage --watch` calls `CollectAll` from scratch
(`internal/usage/watch.go:726`, `renderFrame`), which re-fetches live quota
data for each agent with no retry and a fixed 5s client timeout
(`cmd/harnez/main.go:37`). If that single request fails transiently — a slow
DNS lookup, a momentary network blip, a 5xx from the API — the resulting
frame drops the affected quota windows (`usage.Session`/`usage.Weekly` stay
`nil`; see [issue 031](031-usage-quota-fetch-errors-silent.md)), even though
the previous frame's values are still sitting in `lastSummary` and are very
likely still accurate a few seconds later — quota percentages don't change
that fast.

Concretely: a user left `--watch` running and saw Claude's Session/Weekly
windows missing on the very first frame, then correctly populated on the
next poll a minute later. The dashboard behaved as if the data had never
existed, when in fact it just failed once and briefly displayed less than
what was actually knowable.

## Proposed fix

In `RunWatch`'s `renderFrame` (`internal/usage/watch.go:725`), when a fresh
`CollectClaude`/`CollectAgy`/`CollectCodex` result comes back with a quota
fetch error (per issue 031) and a corresponding window from `lastSummary` is
available, carry the old value forward into the new summary rather than
leaving it `nil` — marked stale (e.g. append `"(stale)"` to the window's
label, or dim it) so the user can tell it wasn't just refreshed. Drop the
stale value once a fetch actually succeeds, or after N consecutive failures,
so a genuinely-gone quota window doesn't get stuck showing ancient data
forever.

Only applies to `--watch`'s live loop; `--summary` and the plain
`harnez usage` report are one-shot and have no prior frame to fall back to —
they should just show issue 031's error line instead.

## Related

- [Issue 031: live quota fetch failures are silent](031-usage-quota-fetch-errors-silent.md) — do this one first; stale-fallback needs the error signal to know when to kick in
- [Issue 023: `harnez usage`](023-usage-command-token-quota-tracking.md) — original command
