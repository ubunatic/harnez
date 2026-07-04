# 012 — Website: feature the TUI demo video

**Status:** 🔴 Open

## Context

A recorded demo exists at `demo-tui-dark.webm` (147 KB, recorded 2026-07-04 via
the `reels/` wayreel scripts), but `website/index.html` is text-only — no visual
proof of the tool in action. Sibling project pages (cati, emojig) lead with a
recorded demo and it is by far their strongest element.

## Proposal

- Copy the demo into `website/` (the subpage must be self-contained — assets
  referenced from `website/index.html` must live inside `website/`, no `../`
  paths; see cati issue 030 for the breakage this prevents).
- Add a hero `<video autoplay loop muted playsinline>` section near the top of
  the page, ideally with a light variant or a theme-neutral recording.
- Regenerate the recording from `reels/` when the CLI output changes.

## Notes

- Also consider a `favicon.png` for the subpage — `uman website status` reports
  no favicon for claudeconfig.
