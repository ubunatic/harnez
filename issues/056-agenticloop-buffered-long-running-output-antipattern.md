# 056 — AgenticLoop.md anti-patterns: add "piping long-running output through a buffering filter"

**Status**: Closed — resolved in d741b99
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

---

## Implementation Plan

### Current state (verified 2026-09-04)

`docs/practices/AgenticLoop.md` section 4 does **not** contain this
anti-pattern. The only `tail` mention in the file is the unrelated
"Orphaned Background Tasks" bullet (`:251`, about `tail -f` watch loops).
055's `Blocking sleep Waits` bullet is present (end of the same list), so the
sibling bullet this ticket wants to sit next to already exists. The proposed
wording in §2 above is still accurate and unapplied.

### Steps

1. Edit `docs/practices/AgenticLoop.md`, section 4 "Anti-Patterns to Avoid",
   inserting the new bullet immediately **after** the `Blocking sleep Waits`
   bullet (the two are the same family — hidden progress — and read best
   adjacent; this ticket's §1 explicitly frames it that way).
2. Use §2's proposed wording, with two additions the draft omits:
   - name `sort`, `wc`, and `head` alongside `tail`/`grep` — any full-input or
     block-buffering filter has the same effect, `tail` is just the common case;
   - give the concrete fix inline: `cmd 2>&1 | tee /tmp/x.log` then read the
     file, or run plain and let the harness auto-background past its timeout.
3. No code changes; `docs/practices/*.md` is a copyable doc, so no `make install`.
4. Resync: `harnez apply` (installs to `~/.claude/docs/AgenticLoop.md`), and
   downstream projects pick it up on their next `harnez init --docs AgenticLoop`
   / `harnez-sync` sweep. Confirm `~/.prime/agent/docs/AgenticLoop.md` is also
   covered by the apply targets before claiming the resync is complete.
5. Verify by grepping the three installed copies for the new bullet text.
6. Close 056. Cross-reference 055 in the bullet so the pair is discoverable.

### Design decisions / tradeoffs

- **Its own bullet, not folded into 055** — as §1 argues: same symptom (no
  progress visible), different mechanism (pipeline buffering vs. turn
  blocking) and different fix. Merging them would make both fixes vaguer.
- **Do not add a lint/hook that rejects `| tail` in Bash calls.** Plenty of
  legitimate short commands pipe to `tail`; the anti-pattern is specifically
  *long-running* commands, which is not statically detectable. Doc-only is the
  right level here.

### Risks / open questions

- Section 4 is getting long; if it keeps growing it may want subheadings
  (waiting/progress, git hygiene, context hygiene). Out of scope here — note
  it and move on rather than restructuring the doc for one bullet.
- The "let the harness auto-background it past its timeout" advice is
  Claude-Code-specific; phrase it as "the harness's background-task mechanism"
  so the doc stays harness-neutral like the rest of the file (which already
  says "the harness's task-management capability" in Phase 4).

### Scope

**Small** — one bullet in one doc plus a resync.
