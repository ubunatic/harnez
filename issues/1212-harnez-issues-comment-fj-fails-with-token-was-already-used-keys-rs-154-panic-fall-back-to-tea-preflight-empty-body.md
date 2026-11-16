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

## Sprint 2026-11-16 (lean sprint 1228, 1212, 1202)

Searcher findings (report: `docs/studies/2026-11-16-two-agent-lean-sprint-forge-filing.md`):

- The comment path checks that the body file exists and is a regular file
  (`cmd/harnez/issues_forge.go:135`) but never reads it; an empty body reaches the forge.
- fj failures: combined output is captured (`cmd/harnez/issues_forge.go:118-123`) and returned in
  the `fj failed` error (`:172-178`); only `401` and "token is required" get an auth hint. The
  "token was already used" text and the `keys.rs` panic are not recognised.
- `tea` is not installed on the owner's machine and there is no fj-to-tea fallback in code.
- The cause of the fj token error is still unverified (fj source and credential store not found).

## Decision

- This sprint delivers the empty-body refusal and the one-line fj auth diagnosis as M2. The
  fallback (tea, or harnez's own API client) waits for the owner's decision on a write-capable
  forge token and is planned with 1202. (Host, 2026-11-16.)

## M2 — refuse empty comment bodies, diagnose the fj token error

Runs after 1228 M1 (shared fj builder).

1. `harnez issues comment`: read the body before any forge call; an empty or whitespace-only body
   (file or stdin) exits non-zero with a clear message and calls no external tool.
2. fj output containing "token was already used" or a `keys.rs` panic: print one line that names
   the fj login state and the fix (`fj auth login`), without the Rust trace. Other fj errors stay
   as they are.
3. Tests with the fake fj: empty file, whitespace-only file, the token error text (assert the
   one-line message and that no trace line is printed), and an ordinary failure left unchanged.

Do not close this ticket: the fallback and the live check are still open.

## M2 delivered: empty-body refusal and fj token diagnosis (`135fb06a`)

`harnez issues comment` reads the body file and refuses an empty or whitespace-only body before
the issue API call and before fj runs; "token was already used" and the `keys.rs` panic give a
one-line login hint. Host check with the installed binary: a whitespace-only file gives
`Error: issues comment: body file is empty or whitespace-only`. Host rerun: `ok
internal/forgetracker`, `ok cmd/harnez`.

### Decision (review of M2, 2026-11-16)

- The body check runs after harnez resolves the git remote locally. That is accepted: the goal is
  "before any forge call", and no network or issue CLI call happens before the check.

### Pre-Work / Required Refinements (do these first in the next developer turn)

1. `TestIssuesCommentRejectsEmptyBody`: also replace `issueCLIRun` with a counter and assert it
   was never called.
2. `TestIssuesCommentFJErrors`: assert the exact one-line login message (see 1228 pre-work 2) for
   both token cases, and the exact preserved text for the ordinary error.
