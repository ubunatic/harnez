# 682 — Configure the Codex statusline with exactly five items

**Status**: Closed — resolved in 1846df62; spec-driven items verified
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #624

---

## 1. Problem & Motivation
The Codex statusline should show exactly these items:

- `current-dir`
- `thread-name`
- `model`
- `context-window-size`
- `context-used`

Other statusline items should not be enabled by Harnez.

## 2. Goal
Configure Codex's statusline to contain exactly the five requested items, with no
unsupported entries or additional items; verify the resulting configuration is
accepted by Codex. Stop and report if blocked on a user decision or denied
permission.

## 3. Implementation & Verification Plan
Codex's ordered item list and retired-item cleanup list live in
`spec/statusline.yaml`, with their allowed values declared in
`spec/schemas/statusline.schema.json`. Apply writes exactly the configured list;
status checks exact content and order; removal clears current and retired Harnez
items while preserving unrelated TUI settings.

Verified with `make install`, `make test-q1`, and `git diff --check`. Codex
0.160.0 loaded a temporary config containing the five item names under strict
config mode; a live TUI launch was unavailable because the isolated probe had no
Codex credentials.
