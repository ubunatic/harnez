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

## M1 canary result (2026-09-30)
- **PASS — manual `/compact` hook and message replacement.** Claude Code 2.1.285 ran with
  `CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1`, the local `--plugin-dir`, Haiku, and a `mktemp` cwd.
  `claude -p --output-format json` created session
  `00965b49-9af2-46eb-a0e0-2d176a3c6828`; resuming it with `/compact` returned
  `local_command: compact`. The saved transcript records a manual `compact_boundary`
  (`preTokens: 16242`, `postTokens: 8`) and a synthetic assistant message containing
  `HARNEZ_646_REPLACEMENT_7f3a91c2`. That marker existed only in the plugin's returned
  `messages`; the next resumed prompt answered with exactly that marker. This proves both the
  hook firing and its returned messages replacing the built-in summary for manual compaction.
- Reproduce with `bash canary/646-session-compact/run.sh`; it validates the CLI result and marker.
  Plugin validation passes. An initial validation caught a forbidden `node:fs` import in the hook
  module; the canary was corrected to log with `$.ui.log` and return the marker directly.
- Auto compaction was not run: this CLI accepts `--autocompact` only at 100k tokens or higher,
  which was not reached in this deliberately small probe. No settings files were edited.
- Evidence session JSONL: `~/.claude/projects/-tmp-harnez-646-HcppcJ/00965b49-9af2-46eb-a0e0-2d176a3c6828.jsonl`.

## Other agents (moved from 644)
- agy: harnez's jev compaction could rewrite agy session data on disk. Canary: does agy accept a
  resumed session whose data was compacted, and does it save tokens compared with agy's own
  auto-compaction? Only then enable it.
- Codex/agy: research whether they offer a real compaction hook (replace messages), like Claude's
  `session.compact`. Findings go here.

## Research: Codex and agy compaction hooks (res646, luna:med, 2026-09-30)
Neither offers a live replacement hook like Claude's `session.compact`.
- **Codex 0.159.1**: `PreCompact`/`PostCompact` hooks exist, notify-only (no replacement-message
  field; codex-rs/hooks/src/schema.rs, events/compact.rs). High confidence.
- **Codex prompt override**: `compact_prompt` / `experimental_compact_prompt_file` apply to local
  compaction only; provider-side remote compaction may bypass them (openai/codex#34428). No
  compaction-model setting.
- **Codex disk rewrite**: `codex resume` loads JSONL rollouts (`~/.codex/sessions/`); resuming an
  edited rollout is undocumented. Medium confidence.
- **agy 1.2.13**: hooks cover tools, model invocation and stop, but no compaction event;
  `PreInvocation` can inject messages, not rewrite history (antigravity.google/docs/hooks). No
  compaction prompt/model setting. Resume picks stored threads; no import of a rewritten transcript.
- **Consequence**: for Codex/agy only two routes remain, each needing its own canary: (a) Codex
  `experimental_compact_prompt_file` for better summaries, (b) disk rewrite + resume.
