# 1226 — harnez agent has no verb to print a session's executed commands for scoring test sessions

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 1221, 1225

---

## 1. Problem & Motivation

`/harnez-cleanup` tells the runner to score test sessions "from the output files and the
harness's command log, never from a summary or transcript the session writes". It names no
command for that log, because none was found: `harnez agent --help` lists no verb that prints
the commands a session executed (checked 2026-11-13 by grepping the help for log, transcript,
show, trace, history and commands; a flag on another verb was not ruled out).

In 1221 the host read the commands out of the provider transcripts with a throwaway script. In
the 1225 run the `terra:high` orchestrator had no such script, so its four test sessions wrote
their own transcripts and the scores rest on self-reported evidence.

## 2. Technical Specification / Findings

- The 1221 script read Claude and Codex JSONL transcripts and printed tool inputs: `tool_use`
  blocks of assistant messages for Claude; `function_call`, `custom_tool_call` and
  `local_shell_call` payloads for Codex.
- Unverified: where harnez records the transcript path per session, and whether the stats
  stream already carries the executed commands.

## 3. Implementation & Verification Plan

- Check first whether an existing verb or flag already prints this; if so, name it in
  `docs/commands/HarnezCleanup.md` and close.
- Otherwise add a read-only verb or flag that prints a session's executed commands, one per
  line, for Claude and Codex sessions.
- Test with a recorded transcript fixture per provider.
- Then name the command in the skill's Baseline section.
