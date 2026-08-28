# 086 — Offline/Degraded Collector Snapshot Is Cached and Served as Fresh, Masking Richer Live Data

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [[082-agent-usage-collector-daemon]], [[085-watch-tui-show-collector-daemon-status]]

## Problem

Found while investigating a user report of "no numbers" in `harnez usage --watch`. Root cause:
during development of [[082-agent-usage-collector-daemon]], `harnez agent-collector --once
--offline` was run as a smoke test, writing a snapshot to
`~/.local/state/harnez/agents/usage/claude.json` that has token totals but **no** quota/session
window data (since `--offline` skips the live Anthropic usage API call).

That snapshot's `fetched_at` timestamp was recent enough to be considered fresh by
`CollectAll`'s cache-first staleness check (< 30 min old), so `harnez usage --watch` served it
as-is — showing token totals but silently missing the Weekly/Session quota bars that a live
collect (or an online daemon run) would have produced. The user perceived this as the tool being
broken ("no numbers"), when the underlying live-collection path actually works fine — confirmed
by manually clearing the cache directory, which restored full quota-bar display via live fallback.

## Desired Behavior

A cache snapshot written from a degraded/partial collect (e.g. `--offline`) should not be
indistinguishable from a full one. At minimum, one of:

- Don't write a cache snapshot at all when the collect was offline/partial (so `CollectAll` falls
  through to live collection or an older, better snapshot).
- Or persist enough metadata in the snapshot (e.g. an `offline: true` / `partial: true` flag) so
  `CollectAll`'s cache-first check can treat it as "usable but not authoritative" and prefer a
  live collect over it, or at least not silently omit fields the user would expect.

## Next Steps

- Decide the exact metadata/skip approach above.
- Add a regression test: an offline-collected snapshot in the cache dir should not suppress
  quota-window data that a live collect would have provided.
