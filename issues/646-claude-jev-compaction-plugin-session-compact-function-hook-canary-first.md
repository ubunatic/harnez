# 646 — claude: jev compaction via session.compact function-hook plugin (canary first)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: [[635-harnez-decide-fast-verbatim-transcript-compaction-explore-picker-skills]], [[645-decide-progressive-state-fitting-and-metric-enhancements-for-verbatim-compaction]], [[644-agent-agy-compaction-never-acknowledged-resume-always-fails]]

---

/goal Replace Claude Code's built-in compaction summary with `harnez compact` (jev, verbatim)
through a small TS plugin using the `session.compact` function hook; canary first; stop and
report if the hook does not fire or cannot replace messages on the installed Claude Code.

## Background
- 635 planned an "optional Claude Code compaction hook plugin"; it was never built (no decision
  against it was recorded).
- The standard `PreCompact` command hook is notify-only: it cannot return replacement messages,
  and Claude summarizes its in-memory copy, so rewriting the transcript file has no effect on the
  live session.
- `tamaratran/fast-jev-compaction` (study: `docs/studies/FastJevCompation.md`) uses
  `on('session.compact', ...)` returning `{ messages }`, plus `turn.complete` to auto-trigger at
  60% context, with fallback to the built-in summary. It calls jev through TypeSafe's hosted API.
- Requirements: Claude Code 2.1.274+ (installed: 2.1.285), early-access flag
  `CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1` in `~/.claude/settings.json`.

## Plan
1. **Canary** (docs/Canary.md): minimal plugin that logs the `session.compact` event and returns
   the messages unchanged. Verify it fires for manual `/compact` and auto compaction, and that
   returned messages replace the built-in summary.
2. Plugin shells out to `harnez compact` (not the hosted API); falls back to built-in on error or
   too little reduction. Thresholds go in `spec/` with a schema.
3. Install via `harnez apply` (flag + plugin), opt-in.

## Other agents (moved from 644)
- agy: harnez's jev compaction could rewrite agy session data on disk. Canary: does agy accept a
  resumed session whose data was compacted, and does it save tokens compared with agy's own
  auto-compaction? Only then enable it.
- Codex/agy: research whether they offer a real compaction hook (replace messages), like Claude's
  `session.compact`. Findings go here.
