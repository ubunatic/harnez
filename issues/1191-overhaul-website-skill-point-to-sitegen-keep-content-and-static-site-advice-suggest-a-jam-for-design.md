# 1191 — Overhaul website skill: point to sitegen, keep content and static-site advice, suggest a jam for design

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**: 1190, 880, 224, docs/commands/website.md, docs/other/Website.md

---

## Goal

/goal The `website` skill and `docs/other/Website.md` match how the owner builds sites today: build
with `sitegen` (`codeberg.org/ubunatic/sitegen/cmd/sitegen`), keep the general advice on what project
content belongs on a website and how to build a robust static page, and suggest a `/jam` when the
design is open, so the owner picks the best version. Done when the skill and doc are updated, stale
instructions are removed, and the root copy is synced. Stop and report when blocked on a user decision
or denied permission; on a loop or no progress, clear the goal and write a failure report here.

## Problem

The skill predates sitegen and may tell agents to hand-build pages. The `web` profile in
`spec/tools.yaml` (1190) was dropped as a placeholder until this lands.

## Scope

- Read sitegen's own README/help first (verify its commands and config; do not guess).
- Skill: when to use sitegen, how to build and preview, where output goes.
- Keep: content checklist (what it does, why, install, demo/proof, links), honest claims only,
  static/no-CDN, relative links, robustness (works without JS, small, accessible).
- Add: for an open design, run `/jam` with several agents and let the owner pick.
- Then re-add a `web` profile in `spec/tools.yaml` with the website doc and sitegen as a tool.

## Open questions

- Does sitegen ship its own agent doc that harnez should link instead of duplicating?
