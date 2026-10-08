# AGY Delivery Route Canary

Issue 758 · 2026-10-08

## Result

**Recommendation: no live route; resume-only.** `agy --conversation <id> -p <message>` appends a turn to the same conversation history, but it runs a separate print-mode process. This probe could not establish delivery into an already-open interactive TUI or interruption of a running turn. Do not treat resume as live session messaging.

## Environment and method

- AGY CLI: `/home/uwe/.local/bin/agy`; model: `gemini-3.6-flash-low`.
- All live probes used scratch directory `/tmp/harnez-758-rdCdMG`; it was removed after the probes. No global AGY config was changed.
- AGY interactive mode requires a TTY. This execution environment has no `/dev/tty`, so the live-TUI and mid-turn checks could not run.
- Latencies below are AGY-reported turn durations. Wall-clock process completion includes CLI startup and varied with tool calls.

## R1 — Resume

Setup command:

```sh
agy --add-dir "$scratch" --model gemini-3.6-flash-low --effort low --output-format json -p 'This is AGY delivery canary 758 setup T1. Reply exactly READY-758-T1 and do not use tools.'
```

Observed: `SUCCESS`, response `READY-758-T1`, conversation ID `b4e6f80a-9680-4ca5-8adc-d0ec94afc7cb`, one turn, 1.21 seconds.

Resume command:

```sh
agy --conversation b4e6f80a-9680-4ca5-8adc-d0ec94afc7cb --add-dir /tmp/harnez-758-rdCdMG --model gemini-3.6-flash-low --effort low --output-format json -p 'canary-758-R1-1 from sender Harnez: reply exactly ACK-758-R1-1, and say whether you recognize a sender name from prior context.'
```

Observed: `SUCCESS`, the same conversation ID, two total turns, 15.66 seconds, response began `ACK-758-R1-1`. It also named Harnez and cited repository/config/MCP context as the source of recognition; this is not evidence of authenticated sender identity. The response was returned to the invoking process. Since no interactive TUI could be started, it was not visible in an open TUI, and idle/mid-turn live delivery is unproven. No second conversation ID was returned.

Interactive TUI attempt:

```sh
agy --new-project --project canary-758-interactive --add-dir /tmp/harnez-758-rdCdMG --model gemini-3.6-flash-low --effort low --prompt-interactive 'Interactive AGY delivery canary 758. Wait for external messages; do not act until one arrives. When a message arrives, reply exactly INTERACTIVE-ACK-758.'
```

Observed immediately: `CLI error: bubbletea: error opening TTY: bubbletea: could not open TTY: open /dev/tty: no such device or address`. No session was created.

## R2 — Hook context injection

Inspected the installed AGY hook documentation at `/home/uwe/.gemini/antigravity-cli/builtin/skills/agy-customizations/docs/hooks.md`. It describes `PreToolUse` JSON input and outputs containing `decision`, optional `reason`, `permissionOverrides`, and `overwrite`. `overwrite` merges into tool-call arguments; no free-form model-context field is documented. Hook input includes `conversationId`, `toolCall`, `stepIdx`, `workspacePaths`, `transcriptPath`, and `artifactDirectoryPath`.

For a live scratch probe, wrote a temporary `.agents/hooks.json` and script that logged stdin and returned:

```json
{"decision":"allow","reason":"canary-758-hook-context","overwrite":{"CommandLine":"printf HOOK-OVERWRITE-758"}}
```

AGY command:

```sh
agy --add-dir "$scratch" --model gemini-3.6-flash-low --effort low --output-format json -p 'For this canary, use run_command exactly once with `printf TOOL-INPUT-758`; then report the command output and tell me if you saw the phrase canary-758-hook-context from a lifecycle hook. Do not perform any other action.'
```

Observed: the first attempt used a relative hook script path and failed with `exit status 127` (`./hook-probe.sh: No such file or directory`); that attempt did not test the output contract. After switching to an absolute script path, the hook ran and AGY reported tool output `HOOK-OVERWRITE-758` plus `A pre-tool lifecycle hook modified the CommandLine argument prior to execution.` The model said it did not see `canary-758-hook-context`. The logged stdin contained the expected conversation and tool metadata. This proves hooks can rewrite a tool call and that the rewrite notice reaches the model; it does not prove a hook can inject an arbitrary message into model context. R2 fails as a general message-delivery route under the documented contract.

## R3 — Native messaging / CLI

Commands inspected:

```sh
agy --help
agy help
agy agent --help
agy mcp --help
agy remote-control --help
agy remote-control status
agy mcp list
```

`agy --help` lists `--conversation`, `--continue`, `--print`, `--prompt-interactive`, `--input-format`, `--output-format`, and subcommands `agent`, `mcp`, `plugin`, `remote-control`, and others. There is no send, queue, inbox, or conversation-list command. `agy agent --help` only lists available agents; `agy mcp --help` only manages MCP configuration. Remote control reported `Daemon status: inactive`. The configured MCP list contained only `harnez stdio /home/uwe/go/bin/harnez mcp`.

**Finding:** no native CLI messaging path or non-interactive conversation-listing command was found. Resume requires an already-known conversation ID.

## R4 — Other routes

The installed CLI help and local AGY guide expose remote control and MCP management, but no app-server command or documented inbox/send operation. With remote control inactive and only Harnez MCP configured, no other live-session delivery route was identified. Harnez MCP availability to an AGY model is not itself a delivery mechanism: it provides no demonstrated inbound polling or notification path to a running session.

## Cleanup and limits

- Removed `/tmp/harnez-758-rdCdMG`, including the temporary hook config, script, and captured input.
- The interactive attempt failed before creating a session. Print-mode probes exited successfully; no probe AGY process remained.
- No global hook or settings file was edited. No code was changed.
- This result establishes same-history print-mode resume and hook tool-argument rewriting only. It does not establish message delivery to a live TUI or detached worker while running.
