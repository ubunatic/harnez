# 352 — Tell agents unrelated untracked files from parallel sessions are expected, not a fabrication concern

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Docs

---

## Summary

Add a short note (project `AGENTS.md`/`AGENTS.local.md`, or a global doc if this is a
cross-project pattern) telling agents: new, unrelated, untracked files appearing in a
repo mid-session are expected and not inherently suspicious — the user runs multiple
concurrent Claude Code sessions against the same working directories, and each session's
own work-in-progress (uncommitted tickets, study docs, template edits) can appear on disk
at any time without warning.

## Background

This recurred three times in one session (2026-09-15):

1. `docs/data/claude-context-debloat.md` appeared after a `harnez index` run, containing
   `/context` output and memory-file listings that didn't match this repo at all.
2. `docs/studies/2026-09-15-claude-code-auto-memory-assessment.md` and
   `issues/350-consider-harnez-memory-command-to-list-clear-claude-code-auto-memory.md`
   appeared later, plus a live edit to `docs/templates/AGENTS.md` — clearly a different,
   parallel session's in-progress work on a related but distinct topic (Claude Code's
   auto-memory feature, not this session's debloat work).
3. Each time, `harnez index` (run to refresh `issues/README.md`/`docs/README.md` after
   this session's own commits) picked up the other session's uncommitted files into the
   generated index tables, requiring a manual diff review and partial revert before
   committing, to avoid committing a reference to content this session didn't author and
   hadn't reviewed.

Handled correctly each time (per existing memory guidance — see
`[[feedback_check_session_trailer_before_flagging_fabrication]]` in the agent's own
memory store, which already covers a *narrower* case: don't assume unexplained commits
are subagent hallucination without checking provenance). But each occurrence required
re-deriving the same conclusion from scratch: notice the mismatch, reason it's probably a
parallel session, decide not to touch/delete it, and manually exclude it from generated
index files. A short explicit note would skip that re-derivation and reduce the chance an
agent instead does something wrong: deletes the file as "unexpected clutter," silently
commits a reference to unreviewed content, or worse, treats it as a prompt-injection
signal and reacts defensively in front of the user for no reason.

## Task

Add a note along these lines (exact wording/placement is an editorial call, not
prescribed here):

> This user runs multiple concurrent Claude Code sessions against the same project
> directories. Untracked files, uncommitted changes, or template/doc edits you didn't
> make may appear at any time — this is normal, not a sign of session compromise, prompt
> injection, or fabricated work. Do not delete or "clean up" files you didn't create and
> don't recognize. Do not silently fold them into your own commits (check `git diff`
> after any auto-generated index/README regeneration — tools like `harnez index` will
> happily pick up another session's uncommitted files into a shared table). If genuinely
> unsure whether something is a parallel session's WIP vs. an actual anomaly, ask the
> user rather than guessing either way.

Consider whether this belongs in this project's `AGENTS.md`/`AGENTS.local.md` (harnez
repo specifically) or in the user's global agent instructions (if this multi-session
habit applies across all their projects, not just harnez) — check with the user which
scope is more accurate before placing it.

## Definition of done

- Note is live in the appropriate AGENTS.md tier (confirm scope with user first).
- Doesn't overlap/duplicate the existing narrower memory guidance about unexplained
  commits and session trailers — this ticket is about untracked *files*, not commits.

## Related

- This session's evergreen cleanup (`docs/CLIDesign.md`, `docs/Permissions.md`,
  `docs/Spec.md` updates, issue 351) — same session where this pattern recurred three
  times.
