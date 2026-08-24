# 056 — AgenticLoop.md anti-patterns: add "piping long-running output through a buffering filter"

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [055](055-no-long-sleep-use-scheduled-wakeups.md)

---

## 1. Problem & Motivation

Found in a downstream project session (lmcoder, 2026-08-24): I ran a
long build/canary command as `make agent-canaries-multi 2>&1 | tail
-60`, intending to trim the eventual output to something readable.
`tail` (like `grep` or any other filter that buffers) doesn't emit
anything until the whole pipeline exits — so a multi-minute command
produced zero visible progress the entire time it ran, indistinguishable
from a hang. The user called this out directly ("your 'make | tail'
kills all output, some progress would be better").

This is the same family of problem [[055]] documents for `sleep`-based
waiting (an agent-side habit that hides progress/wastes the harness's
own notification primitives), but for stdout buffering specifically
rather than turn-blocking. Worth adding as its own anti-pattern bullet
in `docs/practices/AgenticLoop.md`'s "Anti-Patterns to Avoid" list
(section 4) rather than folding into 055, since the mechanism and fix
are different (a pipeline/filter choice, not a wait-primitive choice).

I initially edited the wrong copy of this file — a downstream project's
synced `./docs/AgenticLoop.md` — before realizing it's harnez-managed
and reverting; this project (lmcoder) usually files an issue here
instead of editing `docs/practices/` directly, so filing this rather
than patching it myself.

## 2. Technical Specification / Findings

*(to be filled in during implementation)*

Proposed addition to `docs/practices/AgenticLoop.md` section 4's
"Anti-Patterns to Avoid" list, for reference (not yet applied):

> - ❌ **Buffered Long-Running Output**: Piping a long-running
>   build/test/canary command through `tail`, `grep`, or any other
>   filter that buffers stdout — the filter emits nothing until the
>   whole pipeline exits, so a multi-minute command looks silent/stuck
>   with zero progress visibility. Run it plain (letting the harness
>   auto-background it past its timeout) or `tee` to a file if a
>   trimmed final summary is also wanted.

## 3. Implementation & Verification Plan

1. Add the anti-pattern bullet to `docs/practices/AgenticLoop.md`
   section 4 (wording above, adjust if a better phrasing fits the
   doc's existing voice).
2. Rebuild/resync harnez's bundled docs (`harnez apply` or equivalent)
   so downstream projects' `./docs/AgenticLoop.md` copies (and
   `~/.claude/docs/AgenticLoop.md`, `~/.prime/agent/docs/AgenticLoop.md`)
   pick up the change on their next sync.
