# 1212 — harnez issues comment: fj fails with "token was already used" (keys.rs:154 panic); fall back to tea, preflight empty body

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 1207 (issues comment via fj), 1209 (tea EOL / gt)

---

## 1. Problem

Owner report (2026-11-10): a Haiku agent in `../taia` could not comment on harnez/taia#259 (and earlier #256).
`harnez issues comment -d . 259 -F <file>` failed twice with fj printing `api error: token was already used`
and a `src/keys.rs:154:28` panic trace. The agent then used `tea comments add 259 --repo harnez/taia -d "$(cat file)"`,
which posted. Second issue: the first attempt ran with an empty body file (a heredoc failed in the sandbox
unnoticed) and harnez did not refuse it.

Unverified: cause of the fj token error (stale/one-time token in fj's keys file, or a concurrent fj
refreshing it). Reproduce with `fj` directly before changing code.

## 2. Goal

/goal `harnez issues comment` posts reliably: refuse an empty or whitespace-only body before any forge call;
on the fj "token was already used" / keys.rs failure, print one clear line naming the fj auth state and the
fix (e.g. `fj auth login`), and fall back to `tea` when tea is configured for that host (1209: tea stays as
fallback, gt not adopted). Tests cover the empty body and the fallback with a fake fj/tea. Stop and ask the
owner if fixing needs fj credential changes. On a loop or no progress, clear the goal and write a failure report.

## 3. Acceptance

- Empty `-F` file: non-zero exit, no forge call.
- fj token error: no Rust trace dumped raw; one-line diagnosis; tea fallback used and reported.
- Live check once on a taia test issue (owner OK first).
