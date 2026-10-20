# 833 — usage -w exits on arrow keys; only q and Ctrl-C may quit

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 660

---

/goal In `harnez usage -w`, only `q` and Ctrl-C quit; arrow keys, Esc and other escape sequences (Home, F-keys, mouse) never quit, and a test feeds `\x1b[A` and asserts the watch keeps running. Stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
Pressing Up in `harnez usage -w` exits the app (reported by the user, 2026-10-20). Losing the
dashboard to a stray arrow key is bad UX.

## 2. Technical Specification / Findings
- The terminal sends Up as `ESC [ A`. Keys are dispatched one byte at a time
  (`internal/usage/watch_keys.go`), and on the main dashboard byte 27 (Esc) quits (`dispatchSplashKey`'s
  comment at `watch_keys.go:180-188` says `dispatchWatchKey` owns that), so the first byte of the
  sequence quits before `[A` is read.
- Decided (user): only `q` and Ctrl-C quit. Esc no longer quits; check whether `Q` should stay
  (the user named only `q`) and whether Esc should still close an open overlay.
- Key bindings come from `spec/actions.yaml`; update it and any help/controls text that lists Esc.

## 3. Implementation & Verification Plan
- Parse escape sequences as a unit (or drop them) in the watch key reader.
- Tests: `\x1b[A`, `\x1b[B`, `\x1bOA`, and a lone Esc do not quit; `q` and byte 3 do.
- Live check in a pty: press Up/Down/Esc, the dashboard stays; `q` quits and restores the terminal.
