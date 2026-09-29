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
- **PASS — interactive `/compact` (user, 2026-09-30).** Run via
  `scripts/canary-646-claude-compact.sh`; after a typed `/compact`, Claude reported that the only
  context left was `HARNEZ_646_REPLACEMENT_7f3a91c2` and that no built-in summary was visible.
  `check` passed (compact boundary + marker in the session file).

## M2: jev compaction plugin (embedded)
Pre-Work / Required Refinements:
- **Sandbox check first**: the canary's validation forbade a `node:fs` import in the hook module.
  Find out what the function-hook runtime allows for calling `harnez compact` (child_process?
  a `$` API for exec or fetch? a local HTTP endpoint served by harnez?). If no route exists,
  stop and report before building anything else.
- **Embedded in the harnez binary**: all TS plugin files (manifest, hooks.json, hook module) live
  in the Go tree and are shipped via `go:embed`; `harnez apply` writes them to the plugin
  location and enables the plugin plus `CLAUDE_CODE_ENABLE_FUNCTION_HOOKS`. Opt-in (spec flag),
  idempotent, removable. No hand-maintained copy outside the binary.
- Thresholds (auto-trigger %, minimum reduction, pinned recent messages) go in `spec/` with a JSON
  schema, never as Go or TS defaults; the hook receives them from harnez.
- Fallback to the built-in summary on any error, timeout or too little reduction; log why.
- Tests: Go tests for embed + apply (write, idempotency, removal); a scripted canary run like M1
  that shows a real `harnez compact` result replacing the history.

### M2 bridge probe result (2026-09-30)
- Probe added to `canary/646-session-compact/hooks/probe.ts`. It reports `$` and nested API keys,
  global `fetch`/`process`/`Bun`/`Deno`/`require` types, and attempts `$` exec/shell, loopback
  fetch, and dynamic `node:child_process` import with `harnez --version`.
- Run: `bash canary/646-session-compact/run.sh`, Claude Code 2.1.285, Haiku, `-p`, a `mktemp`
  cwd, `--plugin-dir`, and `CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1`; the script started a Python
  HTTP server bound to `127.0.0.1` and cleaned it up afterward. `/compact` ran, but the debug
  log reports `Registered 0 hooks from 4 plugins` and `Hooks: Found 0 total hooks in registry`.
- **Inconclusive: none of the bridge attempts ran.** The function hook did not register, so this
  run does not establish whether the runtime permits execution or HTTP. Resolve the hook
  registration discrepancy and rerun this probe before proceeding with M2 integration.

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
