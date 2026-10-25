# 912 — usage: show swap in the Load and Remote Load boxes

**Status**: Closed — M1 6ba6af02: swap row in Load and Remote Load boxes; local live run shows swap 1.6/8.0G 20% matching free -m; tests cover swap, no swap and absent fields (older remote: no row); live x600 check moves to epic 921 after 919 upgrades the remote binary
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 885 (disk swap backstop for zram-only systems), 875 (low memory/swap warnings), 090, 879

---

/goal `harnez usage` shows swap usage (used/total, zram and disk swap together) in the local "⁷ Load" box and
in the "[R] Remote Load" box, verified by a test and a live run of the installed binary on this host and the
remote host; or stop and report when blocked on a user decision. Circuit breaker: on a loop or no progress,
clear the goal and write a failure report here.

## Notes

- Owner request (2026-10-24). Swap matters here because the host runs zram-only swap (885) and tmpfs `/tmp`.
- Remote Load renders in `internal/usage/watch_load_box.go:50`; the remote snapshot must carry swap too,
  so check how remote metrics are collected (879) before adding the row.
- Show nothing extra, or "no swap", when a host has no swap; never a silent 0%.
- Check the row against its `.ansi` sketch with the tui-check skill.

## Discovery (searcher s912, haiku:low, 2026-10-25)

- Local: `parseMeminfo` (`internal/usage/load.go:379`) reads only MemTotal/MemAvailable; `SystemMemory`
  (`load.go:53`) has no swap fields; nothing reads SwapTotal/SwapFree yet.
- Render: `buildLoadBoxLines` (`watch_load_box.go:70`), RAM line `formatSystemMemoryLine` (`:176`); remote box
  `buildRemoteLoadBox` (`:45`) uses the same lines.
- Remote: ssh runs `harnez usage --json` (`remote.go:63-74`), parsed by `parseRemoteJSON` (`:136`) into
  `LoadSnapshot` (`types.go:295`). `CPULoad`/`SystemMemory` carry no json tags, so new fields reach the remote
  box with no ssh or cache change. An older remote binary omits them: absent must render differently from 0.
- Tests: `load_test.go:13,37` (meminfo), `watch_test.go:86,134` (memory lines), `:1349,1438,1482` (remote).
- Sketch: `docs/data/harnez-usage.ansi` (Load at :3, Remote Load at :9).

## M1 (swap row, local and remote)

- Parse SwapTotal/SwapFree (zram and disk swap together are what /proc/meminfo reports); add swap fields to
  `SystemMemory` with an explicit "known" flag so a missing value is not 0. Add json tags to the new fields only
  if it does not rename existing keys (stored caches must keep reading).
- Render a swap line under the RAM line in both boxes: used/total; "no swap" when SwapTotal is 0; nothing when
  unknown (old remote binary). Clip to width like the RAM line.
- Update the sketch `docs/data/harnez-usage.ansi` and the relevant lines in `docs/UsageCollection.md`/`docs/TUIDesign.md`.
- Tests: meminfo with swap, with SwapTotal 0, without swap keys; render for each; remote JSON without swap fields.
