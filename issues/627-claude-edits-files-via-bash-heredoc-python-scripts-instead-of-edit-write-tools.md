# 627 — Claude edits files via Bash heredoc/python scripts instead of Edit/Write tools

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Practices / Prompting
**Related**: issue 175 (structured patching), `docs/AgenticLoop.md` anti-patterns (line ~61)

---

/goal Claude sessions in harnez-managed repos edit files with the native Edit/Write tools; harnez-managed
guidance no longer nudges Claude toward shell-based edits. Stop and ask if the fix needs a
Claude-only instruction file change (write it as CxxxE.md) rather than AGENTS.md/docs.

## 1. Problem & Motivation
Observed 2026-09-28 in voxi (Opus 5.5, auto mode): to update ticket 160 Claude chained `sed -i` +
`cat >> file <<'EOF'` + `git commit` in one Bash call, and earlier edited tickets via inline
`python3 - <<'EOF'` scripts. The user had to say "use edit tools". Compound shell edits are hard to
review, fragile, and bundle edit + commit into one permission decision.

## 2. Findings
- `docs/AgenticLoop.md` prefers `apply_patch`/whole-block replacement over narrow `Edit` (11.1% vs 4.2%
  failure rate, from 175). Claude has no `apply_patch`, so this plausibly pushes Claude to shell edits.
- The Claude Code auto-mode system prompt also permits Bash-based edits, reinforcing the habit.
- The auto-mode classifier "no verdict" failures that blocked the shell edits also hit Write/Edit in the
  other repo while filing this ticket; writing to the session scratchpad and moving the file worked.
  So blocking was partly environmental; the habit problem stands on its own.

## 3. Implementation & Verification Plan
- Make the patching guidance agent-aware: Claude → Edit/Write (multi-line `old_string` blocks count as
  whole-block replacement); Codex → `apply_patch`. Keep edits and commits in separate tool calls.
- Verify: a fresh Claude session in a harnez repo, asked to update a ticket, uses Edit/Write.
