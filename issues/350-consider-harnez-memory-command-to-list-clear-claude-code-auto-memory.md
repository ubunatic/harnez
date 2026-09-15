# 350 — Consider a `harnez memory` command to list/clear Claude Code auto-memory

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Tooling / Agent Ergonomics
**Related**: `docs/studies/2026-09-15-claude-code-auto-memory-assessment.md`

---

## 1. Problem & Motivation

Claude Code's "auto-memory" feature writes per-project files to
`~/.claude/projects/<project-slug>/memory/` (an always-loaded `MEMORY.md`
index plus on-demand detail files). This state lives outside any repo
harnez tracks and has no first-party cleanup path:

- No `claude` CLI subcommand lists, shows, or clears memory
  (`claude --help` has no `memory`/`clear`/`reset`/`forget` verb for it).
- `claude --bare` only *disables* auto-memory for one invocation; it doesn't
  clear existing files.
- The only way to remove stale or wrong entries today is manual
  `rm -rf ~/.claude/projects/<slug>/memory/*` (or hand-deleting individual
  `*.md` files) with no reindex step to re-sync `MEMORY.md` after.

A survey of the 26 project memory dirs on this machine (see the linked
study) found most are empty (17/26) and the rest are small (largest is
harnez at 44K / 11 files), so this is not an urgent pain point — but there
is currently no supported way to correct memory once an agent writes
something wrong or outdated into it, and no visibility tool short of `ls`
+ manual reading.

## 2. Scope Questions (resolve before implementing)

1. **Claude-only or cross-agent?** This ticket and its background study only
   assessed Claude Code. This repo also manages Codex and other agent
   harnesses (see `docs/commands/HarnezAdvisor.md`, `internal/usage/*.go`
   provider files) — if a `harnez memory` command is built, decide whether
   it targets Claude Code's memory layout specifically or needs a
   provider-abstraction similar to `internal/usage/` before it can
   generalize. Do not assume other agents share Claude Code's
   `~/.claude/projects/<slug>/memory/MEMORY.md` + detail-file layout.
2. **Command shape**: candidate subcommands are `list` (per-project memory
   file inventory with sizes), `show <project>`, `clear [--project <dir>]`,
   and `prune-empty` (remove the 17 currently-empty `memory/` dirs found in
   the survey). Confirm which of these are actually wanted before building
   more than the minimum.
3. **Fits which existing surface?** Decide whether this belongs under
   `harnez find`/`harnez issues`-style discovery commands, a new top-level
   `harnez memory` verb, or stays a manual filesystem operation permanently
   documented in a doc instead of tooled.

## 3. Acceptance Criteria (draft — refine once scope above is settled)

- [ ] Scope question 1 answered and recorded in this ticket before any code
      is written.
- [ ] If built: a read-only listing command works across all populated
      project memory dirs without requiring `cd`.
- [ ] If built: a clear command requires an explicit project target (no
      accidental global wipe) and reports what was removed.
- [ ] Empty-`memory/`-dir pruning, if included, is safe to run repeatedly
      (idempotent, no-op when nothing to prune).

## 4. Recommendation

Low urgency — file and leave in backlog rather than building now. No
observed incident of stale/incorrect memory causing bad agent behavior in
this repo; the study that produced this ticket was a proactive audit, not a
response to a failure.
