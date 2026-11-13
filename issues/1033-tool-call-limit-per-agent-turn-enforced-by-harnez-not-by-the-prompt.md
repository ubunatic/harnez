# 1033 — Tool-call limit per agent turn, enforced by harnez, not by the prompt

**Status**: Open — M1–M3 implemented and tested in c6d9a828 (feat/jules-batch-b); live lmcoder:coder acceptance deferred (pi absent / proxy 127.0.0.1:8735 offline)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1024, 1027, 1034, 1023, 592, 241, 662

---

## Problem
A small model can loop on tool calls instead of answering. In 1024 (2026-10-29, `lmcoder:coder`, 64k window) a
review of a 98-line diff made 73 tool calls with an open prompt and 80 with a prompt that said "at most 4 more
tool calls" (51 `grep`, 30 of them repo-wide). Both runs ended at the 10-minute cap without an answer. A limit in
the prompt is not followed, so it must be enforced by the harness.

## Ask
A per-turn tool-call limit for `harnez agent start`/`resume`: when a turn reaches the limit, harnez stops further
tool calls and asks the model to answer now with what it has; if it still does not answer, the turn ends with a
clear "tool-call limit reached" status, not a bare timeout.

"Turn" here means one `harnez agent start` or `resume` prompt, which is one provider process. It does not mean
Pi's `turn_start`/`turn_end`, which mark a single assistant response.

## Findings (live code and Pi 1.4.0 source, read 2026-10-31; nothing below has been run yet)

**Harnez today**
- The only live observer is `TokenWatchdog.Observe`, called from the stream callbacks at
  `cmd/harnez/agent_run.go:506` (start) and `:877` (resume). On a crossing it cancels the context, and
  `providerLines` then sends SIGKILL to the process group (`internal/subagent/process.go:56-60`). `StopGroup`, the
  SIGINT-then-SIGKILL path, is not used by `agent_run.go`.
- **Trap: `--json` turns have no live events.** `streaming = streaming && !req.JSON` (`agent_run.go:486`) sends them
  through the batch `driver.Run`. The detached worker (`agent_run.go:460`) and the MCP commands
  (`internal/mcp/server_test.go:71`) both pass `--json`, so most subagent runs would escape a stream-based limit.
  `TokenWatchdog` has the same gap today.
- **Stream parsers drop tool calls.** `piLineFunc` (`pi.go:83`) handles only `session` and `message_end`. Codex maps
  only `command_execution` to an event (`codex.go:458`), so other item types are not counted. Claude emits
  `tool_use` as `activity` for the main thread only (`claude.go:281`) and does not keep the block id.
- **Agy has no `RunStream`**, so nothing can count live. Its `harnez hook agy` PreToolUse hook has matcher `*`
  (`internal/agy/hooks.go:70`) and is the only possible route.

**Pi 1.4.0** (`pi-agent-core/dist/agent-loop.js`, `pi-coding-agent/dist/core/extensions/types.d.ts`)
- The `tool_call` handler can return `{block, reason, terminate}` (`types.d.ts:1089`). The model receives `reason` as
  an error tool result (`agent-loop.js:514`).
- `terminate` ends the agent loop after the batch only when every call in that batch set it (`agent-loop.js:472`).
- Calls in a parallel batch are prepared one at a time (`agent-loop.js:419-425`), so counting is deterministic.
- `tool_execution_start` is emitted before the hook, so harnez can also count blocked calls from the stream.
- **Trap: do not use `setActiveTools([])`.** A call to a removed tool returns "Tool not found" before the hook runs
  (`agent-loop.js:489`), so `terminate` never fires and a model that keeps calling tools can loop.
- **Trap: a terminated run looks like success.** It ends with an assistant message that has no text, and `parsePi`
  then returns a "completed" result with an empty response (`pi.go:292-318`).
- `pi --extension` paths still load under `--no-extensions` (`pi --help`), so this works with the 1027 `plain` mode.

**Claude and Codex**
- `claude --help` (2.1.316) lists no `--max-turns`; `--max-budget-usd` limits money, not calls.
- Both have PreToolUse hooks, but the harnez Codex hook that rewrites commands and the Claude exec hook match only
  `Bash`. Blocking every tool needs a wildcard hook plus a per-session counter, in the user's global config or
  through per-run settings. This ticket does not do that.

## Decisions

**Two enforcement layers, one result**

| Provider | Blocks calls with | Asks for the answer by | Live count from |
| --- | --- | --- | --- |
| `pi`, `lmcoder` | Extension: `tool_call` block, then `terminate` | Block `reason` to the model | `tool_execution_start` |
| `claude`, `codex` | Harnez: stops the process group | One extra `Resume` with the synthesis prompt | Stream events |
| `agy` | Nothing (see below) | Nothing | None |

**Limiter.** A new `internal/subagent/toollimit.go` holds a small `ToolCallLimiter` next to `TokenWatchdog`:
`Observe(Event)` counts calls and says when to stop. `Event` gets a `ToolCall bool` field and a call id for
de-duplication, so existing `Kind` handling and the output stay unchanged. The limiter is the single source of the
count; drivers do not count separately in `parse*`.

**Semantics.** With limit N, N calls run and call N+1 is the first one blocked. `0` means off.
`tool_calls` reports attempted calls, including blocked ones.

**Behaviour at the limit: graceful first, then a hard stop.**
1. Pi: calls over the limit in the batch that crosses it are blocked with the synthesis prompt as `reason`, without
   `terminate`. The model gets one more response. Any tool call in a later response is blocked with `terminate`.
2. Claude and Codex: harnez cancels the turn, then runs one `Resume` with the synthesis prompt and a budget of zero:
   the first tool call in that run ends it. This is the same cancel the token watchdog uses.
3. Outcome, the same for all providers:
   - An answer arrived: `Status=completed`, the response is the answer, and `tool_call_limit_reached` is true.
   - No answer: `Status=stopped`, a non-zero exit, and
     `LastError="tool-call limit reached: <attempted> calls (limit <N>), no answer; session can be resumed"`.
     Any partial text stays in `Messages` and is not presented as the answer.
   `parsePi` must not report the empty terminated run as completed.

**Pi backstop.** The generic limiter also runs on Pi at `limit + native_backstop_calls`. If the extension did not
load or did not enforce, harnez stops the process and the status says so, without a synthesis step.

**Pi extension delivery.** A single-file, self-contained `tool-call-limit.ts` is embedded in the binary with
`go:embed`, written atomically to a content-hashed path in the harnez state dir, and passed as `pi -e <path>` only
when the limit is above 0. It is not installed by `harnez apply`, so it cannot drift from the binary and does not
affect plain Pi sessions. The source must stay out of `.pi/extensions/`, which autoloads. It reads the limit and
prompt from `HARNEZ_TOOL_CALL_LIMIT` and `HARNEZ_TOOL_CALL_LIMIT_PROMPT`, set the way `setAgentEnv` does, and has no
defaults of its own. A `withToolCallLimit(driver, n)` wrapper sets a `PiDriver` field, as `withAgyMeterSession` does,
because `Resume` takes no options.

**Agy.** Not enforced in this ticket. With a limit above 0, harnez prints a warning that the limit is not enforced
for this provider, and `tool_calls` is omitted rather than shown as 0. This uses the goal's "stop and report" clause;
wiring the agy hook needs its own canary.

**`--json` turns.** When a limit is active and the driver supports streaming, harnez calls `RunStream` with an
observer that prints nothing, so the JSON output stays unchanged. Moving `TokenWatchdog` onto this observer would close
its gap too, but it is out of scope here unless it comes for free.

**Configuration** (`spec/agent.yaml` with `spec/schemas/agent.schema.json`, no Go defaults):

```yaml
tool_call_limits:
  default: 0                    # off: paid models legitimately run long developer turns
  models:                       # same key lookup as compact_thresholds: spec, provider:name, provider
    lmcoder:coder: {default: 40, advisor: 15, reviewer: 15}
  native_backstop_calls: 20
  synthesis_prompt: "Tool-call limit reached. No more tools are available. Answer now with what you have, and say what you could not check."
```

Precedence: `--max-tool-calls N` > `~/.harnez/config.yaml` > spec, each at model and then role level. The starting
numbers come from 1024, about 8 s per call (73 calls in 10 min), so 15 calls take about 2 minutes. M3 checks them.

**Flag.** `--max-tool-calls N` on `agent start` and `agent resume`, for that run only, not stored in the session.
It must be forwarded in the detached worker args (`agent_run.go:460` and the resume equivalent). The MCP `harnez_command`
and `harnez_spawn_agent` tools get a `max_tool_calls` property. Interactive runs (`-i`) reject the flag.

**With other tickets.** 1027 limits which tools exist, this limits how many calls; they are independent. 1034 runs
have no tools, so the limit never fires. Subagent (Task) calls in Claude do not count, because the stream excludes them.

## Milestones

**M1: spec and core limiter (no provider change).**
- Spec, schema, loader validation, and a `ToolCallLimit(model, role)` lookup with the precedence above.
- `ToolCallLimiter`, `Event.ToolCall`, `--max-tool-calls`, worker and MCP forwarding, the silent observer for `--json`.
- `tool_calls`, `tool_call_limit` and `tool_call_limit_reached` in `TurnResult`, `TurnRecord`, the agent JSON output
  and the final stream summary.
- Done when: unit tests cover lookup, precedence and limiter thresholds, and `--json` output is otherwise unchanged.

**M2: provider integrations.**
1. Canaries first (`docs/Canary.md`), with results recorded here:
   - Pi: block reason leads to an answer; `terminate` exits 0 with `agent_settled` not aborted; `-e` loads under
     `--no-extensions`.
   - Claude and Codex: a session resumes after SIGKILL during a tool call, and which Codex item types are tool calls.
   If Claude or Codex cannot resume after the kill, that provider gets the hard stop without synthesis and the status
   says so.
2. Pi: `tool_execution_start` in `piLineFunc`, the extension and its delivery, the `parsePi` fix, the backstop.
3. Claude and Codex: stream `ToolCall` events (with Claude's block id), cancel plus synthesis `Resume`.
4. Agy: the warning and omitted count.
5. Docs: `docs/PiAgent.md` and `docs/HarnezAgentArchitecture.md`.
- Done when: each provider row in the table above behaves as written, or its deviation is recorded here.

**M3: tests and live verification.**
- Go, real process: a fake provider script on `PATH` that emits tool events until killed, proving the live stop on
  both the streaming and `--json` paths. The `Command` override cannot prove this, because it replays output after the
  process has finished (`process.go:102-105`). Wait on output, with no sleeps (`docs/TestTiers.md`).
- Go, parser: `parsePi` on a terminated run, de-duplicated call ids, synthesis answered and not answered.
- Extension: node tests with a fake `pi` object in `scripts/test-pi.sh`: under the limit, crossing inside a batch, the
  `terminate` stage, invalid env.
- Live: the acceptance run below. Set the 1024 numbers in the spec from its result. If the proxy reports `coder`
  `live: false`, as it did during 1024, that part stays open and is reported, not skipped silently.

## Acceptance
- The 1024 review (diff `38190e98^..f27aeebd`) on `lmcoder:coder --role reviewer --max-tool-calls 10` ends with an
  answer or the clear limit status within a few minutes, with the call count in the run output.
- Tests with a fake provider that keeps calling tools pass on the streaming and `--json` paths.
- `make test-short` and `make test-pi` pass. With no limit configured, provider arguments and output are unchanged.

/goal Enforce a per-turn tool-call limit as specified above, at least for Pi/lmcoder, or stop and report when a
provider gives no way to block or count calls. Circuit breaker: on a loop or no progress, clear the goal and write a
failure report. Re-check live code and Pi's version first (Findings are from Pi 1.4.0); refined 2026-10-31.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| implement (M1) | jules-impl-1033 | 10891694979554508594 | github #170 (closed) | stalled after 2 replies, but the PR arrived later; partly taken by host (see Progress) |

## Progress

- M1 part 1 (2026-11-11, host from Jules GH#170): `tool_call_limits` in `spec/agent.yaml` and schema,
  Go validation (an absent section means off), `ToolCallLimiter` (`internal/subagent/toollimit.go`; a repeated
  call id repeats its first decision), `Event.ToolCall`/`CallID`, and the spec lookup
  `ToolCallLimitsSpec.Limit` / `ToolCallLimit(model, role)` (`toollimit_spec.go`): the first matching model key
  (spec, provider:name, provider) wins, then the role, then its `default`, else the section default. The PR's
  lookup test re-implemented the lookup inside the test; replaced by tests on the embedded spec. Advisor
  review (codex:luna:low): the repeated-id defect was fixed; "schema rejects an empty section, Go accepts it" is
  accepted, since Go cannot tell an empty section from an absent one and the schema check covers it.
- M1 left: `--max-tool-calls` with precedence over `~/.harnez/config.yaml`, worker and MCP forwarding, the
  silent `--json` observer, and `tool_calls`/`tool_call_limit`/`tool_call_limit_reached` in `TurnResult`,
  `TurnRecord` and the output.
- M1 complete, M2 and M3 (fake-provider + node extension tests) implemented (2026-11-13):
  - Config & flags (`toollimit_spec.go`, `agent.go`, `agent_cmds.go`, `agent_interactive.go`, `agent_async.go`, `agent_run.go`, `internal/mcp/server.go`):
    `EffectiveToolCallConfig` enforces `--max-tool-calls` > `~/.harnez/config.yaml` `agent.tool_call_limits` > `spec/agent.yaml`;
    `-i` rejects `--max-tool-calls`; detached workers and MCP (`harnez_spawn_agent`, `harnez_command`) forward it;
    `--json` runs upgrade to `StreamingDriver` silently (`useStream` vs `streaming`) when limit > 0.
  - Drivers & Pi extension (`tool-call-limit.ts`, `toollimit_embed.go`, `pi.go`, `claude.go`, `codex.go`):
    embedded `tool-call-limit.ts` written atomically to content-hashed path in `$XDG_STATE_HOME/harnez/pi/` and passed via `pi -e <path>` with `HARNEZ_TOOL_CALL_LIMIT` / `HARNEZ_TOOL_CALL_LIMIT_PROMPT` only when limit > 0;
    `parsePi` returns `*PiEmptyResponseError` preserving partial `Messages` on terminated empty runs; `piLineFunc` emits `tool_execution_start`;
    `claudeStreamer` includes `tool_use` block `ID` on main-thread calls; `codexParser` emits tool item events (`command_execution`, `mcp_tool_call`, `file_change`, `web_search`, `collab_tool_call`, `custom_tool_call`, `local_shell_call`, `*_call`).
  - Enforcement & output (`agent_run.go`, `agent_stream.go`, `driver.go`, `session.go`):
    `toolCallGuard` wraps `ClaudeStreamDriver` / `PiStreamDriver` when limit > 0, runs Pi backstop at `limit + native_backstop_calls`, cancels Claude/Codex and runs one synthesis `ResumeStream` cancelling on first tool call, records `tool_calls`/`tool_call_limit`/`tool_call_limit_reached` on `TurnResult`, `TurnRecord`, JSON output, and `[done: ...]` stream tail, and saves `Status=stopped` with `LastError="tool-call limit reached: <attempted> calls (limit <N>), no answer; session can be resumed"` on no answer.
    `agy` warns on stderr and omits `tool_calls`.
  - Docs updated (`docs/PiAgent.md`, `docs/HarnezAgentArchitecture.md`).
  - Environment probe & live verification status: `pi` binary is absent on `PATH`, `HARNEZ_PI_PACKAGE_DIR` is unset, and `127.0.0.1:8735` (`lmcoder` proxy) is not listening in this session. Live `lmcoder:coder` acceptance run (1024 review diff) could not run here and remains open until run against a live Pi + `lmcoder` environment; unit tests, `test/pi/tool-call-limit.test.ts` (`make test-pi`), and real-subprocess fake-provider CLI tests (`cmd/harnez/agent_toollimit_test.go`) all pass.
