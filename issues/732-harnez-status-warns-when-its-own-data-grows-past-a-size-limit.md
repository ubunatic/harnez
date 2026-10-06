# 732 — harnez status warns when its own data grows past a size limit

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [#343 host disk meters](343-display-host-disk-usage-and-storage-meters-in-load-telemetry-tui.md), [#404 storage metrics](404-add-hdd-ssd-storage-and-i-o-usage-metrics-to-usage-watch-tui.md)

---

## 1. Problem & Motivation
A bug in the legacy usage archive (fixed in `2f4f946e`) copied all usage files
on every refresh: 7,642 snapshots, 166 GiB apparent (about 74 GiB on disk) in
`~/.local/share/harnez/archive/usage-legacy/` within a week. Nobody noticed
until the home disk ran low, because nothing reports how much space harnez
itself uses. #343 and #404 cover host disk meters, not harnez's own data.

/goal `harnez status` reports the size of harnez's own data directories and
warns, with the path and a hint, when one exceeds its limit. Limits live in
`spec/` with a schema (see `docs/Spec.md`), not as Go defaults. Stop and ask
before adding any automatic cleanup.

## 2. Technical Specification / Findings
- Candidates: `$XDG_DATA_HOME/harnez` (archive, telemetry DB, usage
  history), `$XDG_STATE_HOME/harnez`, `$XDG_CACHE_HOME/harnez`, `~/.harnez`.
- On btrfs, `du` reports uncompressed sizes; apparent size is fine for a
  warning, but say which one is shown.
- Status must stay fast: walking 7,000+ directories took seconds. Bound the
  walk or cache the result.

## 3. Implementation & Verification Plan
- Test with a temp data dir above and below the limit.
- Live check: `harnez status` on this machine shows sizes and no warning.
