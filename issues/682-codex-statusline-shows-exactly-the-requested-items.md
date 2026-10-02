# 682 — Configure the Codex statusline with exactly five items

**Status**: Open
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
Update the Codex statusline configuration and its tests, then run the focused
verification and confirm the installed Codex accepts all five item names.
