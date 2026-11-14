# 1226 — harnez agent has no verb to print a session's executed commands for scoring test sessions

**Status**: Closed — cc11b4f1/1dee4d1a: add harnez agent commands for Claude and Codex session transcripts; fixture + CLI tests; docs updated; live verified on codex:luna:low
**Closed-By**: session 801d927d-19ac-46f0-aece-6142997b71d9
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
- Found after filing (the host had not read it first): `docs/OrchestratedAgentFlow.md`,
  section 5 "Finding out what agents ran", already names the Codex rollout logs and the
  telemetry database (`~/.harnez/tool_catalog.sqlite`, `harnez stats`) as sources. The Claude
  transcript path was added there on 2026-11-13. So the data is reachable by hand; what is
  missing is one command an orchestrator can name in a task. Whether `harnez stats` can list one
  session's commands was not checked.

## 3. Implementation & Verification Plan

- Check first whether an existing verb or flag already prints this; if so, name it in
  `docs/commands/HarnezCleanup.md` and close.
- Otherwise add a read-only verb or flag that prints a session's executed commands, one per
  line, for Claude and Codex sessions.
- Test with a recorded transcript fixture per provider.
- Then name the command in the skill's Baseline section.

## Decision
- Add read-only subcommand `harnez agent commands [<id>] [--name <name>] [-d <dir>]` (accepting Harnez session id/prefix or `--name`, resolving via the session store to the provider session/thread ID and transcript path) printing executed shell commands one per line.
- Share provider command extraction in `internal/` (handling Claude assistant `tool_use` `Bash` blocks and Codex rollout `function_call`, `custom_tool_call`, `local_shell_call` payloads) and verify with unit tests using recorded Claude and Codex JSONL fixtures (plus a live Codex test session before merge).
- Update `docs/commands/HarnezCleanup.md` Baseline section and `docs/OrchestratedAgentFlow.md` §5 to name `harnez agent commands` (updating copyable source/root doc copies if applicable).

## Pre-Work (Scout findings)
- Confirmed gap: `cmd/harnez/agent*.go` has no verb printing executed shell commands; `harnez stats --session <id> --calls` (`cmd/harnez/stats.go:97`, `cmd/harnez/stats_calls.go:52`) prints telemetry rows without shell command strings, and `docs/OrchestratedAgentFlow.md:135` (§5) relies on raw transcripts / `~/.harnez/tool_catalog.sqlite`.
- Session & transcript lookup:
  - Resolve Harnez agent ID/prefix or `--name` via `cmd/harnez/agent_manage_cmds.go` conventions to provider session/thread ID and working dir.
  - Claude transcripts live under `~/.claude/projects/<encoded-cwd>/<session-id>.jsonl` (`internal/usage/transcripts.go:9`, `internal/transcript/extract.go:28`, `internal/transcript/claude.go:28`); extract assistant content blocks with `type == "tool_use"` and `name == "Bash"` (`input.command`).
  - Codex rollouts live under `~/.codex/sessions/YYYY/MM/DD/rollout-*-<thread-id>.jsonl` (`internal/subagent/codex.go:51` `findCodexRolloutPath`, event/call parsing at `internal/subagent/codex.go:527`, `:673`, and fixture `internal/subagent/testdata/codex-987-rollout.jsonl`); inspect live Codex rollout event schema for `exec_command` / `shell_command` / `function_call` / `custom_tool_call` / `local_shell_call` payloads so real sessions and fixtures both parse cleanly.
- Docs check: `docs/commands/HarnezCleanup.md` is an evergreen root doc (verify no `harnez:variant=lite` / copyable source exists); document `harnez agent commands` in its Baseline section and in `docs/OrchestratedAgentFlow.md` §5.
- Verification: `go test -race -count=1 ./cmd/harnez ./internal/subagent ./internal/transcript` + live `codex:luna:low` test session check after `make install`.

### M2 Pre-Work (Reviewer findings on `cc11b4f1`)
- `internal/transcript/commands.go`: support real Codex `custom_tool_call` with `name: "exec"` whose `input` executes JS `tools.exec_command({cmd: ...})` (and multiple calls / string-literal variants inside an `exec` block; inspect real Codex rollouts like `/home/uwe/.codex/sessions/2026/11/14/rollout-2026-11-14T04-07-12-01a1d805-fa4f-7ea2-8cc4-c71b6aa15a5f.jsonl` to confirm exact payload forms, including `shell_command`/`shell` if wrapped).
- `cmd/harnez/agent_commands.go` & `cmd/harnez/agent_args.go`: reuse `resolveSession` cleanly instead of duplicating `store.List` + prefix search; do not run `os.Stat` path shortcut on `--name` (or bypass path shortcut when `--name` is set, and only treat positional arg as file path when appropriate, respecting `--dir` cleanly).
- `internal/transcript/commands_test.go` & `cmd/harnez/agent_commands_test.go`: add recorded/real-shape fixture cases for Codex `custom_tool_call` (`name: "exec"`, multiple/multiline `tools.exec_command`), CLI Claude transcript case, CLI `--dir` filtering, `--name` when a matching file exists in cwd, and CLI multiline `\r`/`\n` output verification.
- Verify with `go test -race -count=1 ./cmd/harnez ./internal/subagent ./internal/transcript` and commit code+tests with `(#1226)`.

**/goal** Implement `harnez agent commands` to list executed shell commands from Claude and Codex session transcripts with recorded fixture tests and doc updates, or stop and report when blocked on a user decision or denied permission.
