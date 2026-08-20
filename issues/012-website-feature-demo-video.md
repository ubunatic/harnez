# 012 — Website: feature the TUI demo video

**Status:** 🔴 Open (Blocked on clean recording & user verification)

## Context

A recorded demo exists at `demo-tui-dark.webm` (147 KB, recorded 2026-07-04 via
the `reels/` wayreel scripts), but inspection revealed recording bugs:
- Antigravity appears started with simulated keystrokes, but no letters render and no AGY UI is shown.
- Automatic embedding of unverified demos violates the media verification rule.

## Proposal

- Re-record a clean demo reel of `harnez usage --watch` or interactive commands without recording artifacts.
- Copy the verified demo into `website/` (self-contained, no `../` paths).
- **Media Verification Gate**: Always ask the user for confirmation that the visual demo content matches expectations before publishing to `website/index.html`.

## Status Notes

- Video embed removed from `website/index.html` until a clean recording is produced and confirmed by the user.
- Website text, narrative (Problem → Approach → Result), and command table updates remain active.
