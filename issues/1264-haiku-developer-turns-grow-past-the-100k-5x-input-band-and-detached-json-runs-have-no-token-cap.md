# 1264 — Haiku developer turns grow past the 100k 5x input band, and detached --json runs have no token cap

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 591, 592 (closed), 969, 1033, 956, 973; study `docs/studies/2026-11-21-headless-signal-assistant-one-day-build-story.md` (table row 9)

---

## 1. Problem & Motivation

Haiku 5.5 input above 100k tokens per request costs 5x (`docs/AutoCompaction.md:94`, citing the Anthropic pricing page; `spec/agent.yaml:224`). In the 2026-11-21 headless run, Haiku developer turns ended with context usage above that band: sig-voice 151.0k, sig-voice-fix 145.7k, sig-poll-fix 143.5k, hub-006b 115.0k, sig-poll 112.3k, sig-rich2 112.2k (study table row 9, line 131). Haiku's compaction threshold was 80k then; it is 100k since 2026-11-27 (`spec/agent.yaml:45-48`). The study records no cap that stopped these turns.

## 2. Technical Specification / Findings

- **Compaction is between turns.** `docs/AutoCompaction.md:13-15`: the check runs "before each prompt". A single turn can grow past 80k with no interruption.
- **The token watchdog (592, closed) needs the stream.** `Observe` runs only inside `if streaming && thresholdErr == nil` (`cmd/harnez/agent_run.go:714-715`). `streaming := canStream && !req.JSON` (`:677`, and `:1127` for resume).
- **Detached children run with `--json`.** The spawned child gets `--json` at `cmd/harnez/agent_run.go:635`, `cmd/harnez/agent_async.go:304` and `cmd/harnez/agent_run.go:1145`. So a detached Haiku developer has no token watchdog. Not verified for the orchestrator's own invocation.
- **The tool-call limiter is different.** `useStream := canStream && (streaming || tcGuard != nil)` (`:678`) forces the stream path when a tool-call limit is set, so `--json` turns are observed by the limiter. Its default is off (`spec/agent.yaml`, `tool_call_limits.default: 0`; 1033).
- **A warning does exist.** The cost check prints `[check: ...]` (`spec/agent.yaml:23-26`, `cmd/harnez/agent_stream.go:309`). The study records one such line for a turn: "turn re-read 504.0k cached for 10.9k new (46x)" (study line 131). Nothing stops the turn. Whether the check prints on the `--json` path is **unverified**.
- **Claude `-p` ignores the percent trigger** (measured, `docs/AutoCompaction.md:42-43`). The 80k harnez gate is the only Haiku trigger.
- **Unverified:** why turns ended at 112k-151k when the threshold is 80k. Either no compaction ran between turns, or one turn grew past 80k. The per-turn peak is not in the study.

## 3. Implementation & Verification Plan

Options, in order of value:

1. Make the token watchdog work for detached runs: observe the child's events, or run the child in stream mode while the parent keeps `--json`.
2. Warn at the 100k band for Haiku at turn end and in the `[done: ...]` line. New key in `spec/agent.yaml`, documented per `docs/Spec.md`.
3. Owner decision: set `tool_call_limits` for Haiku developer roles (1033).

Canary before any build (repo rule: canary first): one detached Haiku turn with a prompt above 100k tokens, to measure whether the 80k cap fires. Run 2026-11-27 with the owner's approval: see section 5.

## 4. Notes

- **2026-11-22, hub-run (C): the 80k gate does not bound a single turn.** The orchestrator's hub run had Haiku workers reach about 167k context inside one long turn. Harnez's gate runs only before a prompt (`docs/AutoCompaction.md:13-15`), so nothing in harnez stopped the turn. The CLI's own auto-compaction did not hold it under 100k either: `docs/AutoCompaction.md:38` gives the Claude Code autocompact buffer as 33k by default, so the trigger sits near the window size minus 33k. If Haiku's window is 200k (unverified here), that gives about 167k, which matches the report. Source of the 167k figure: the orchestrator's hub log, not in this repo. Verify with the canary in section 3 and read the stream's usage events for the turn.

## 5. Canary (2026-11-27, owner approved the spend)

One read-only Haiku (`claude:haiku:med`) turn, the same prompt twice: read five files (10,729 lines) with `harnez read -n -L` in 500-line ranges, then list their last line numbers. The effective Haiku threshold was 100000 (spec and `~/.harnez/config.yaml`). Per-request prompt sizes come from the canary transcripts (usage numbers only).

| Run | Mode | Requests | Prompt per request (k) | Over 100k | Claude auto-compacts | harnez watchdog | Cost |
|---|---|---|---|---|---|---|---|
| canary-1264b-stream (455096c9) | streaming | 8 | 20, 80, 134, 86, 147, 169, 35, 43 | 3 | 2 (preTokens 165791, 168571) | **did not fire** | n/a in output |
| canary-1264b-json (c13d8a9a) | `--json` | 9 | 20, 80, 134, 91, 148, 170, 35, 43, 65 | 3 | 2 (preTokens 176133, 177303) | not armed (expected) | 191341 µ$ |

Findings:
- **The watchdog does not stop a streaming turn either.** The turn passed 100k at request 3 (134k) and ran to completion. Neither `[check: ...]` nor any threshold message appeared. So the defect is not only the `--json` gate.
- **Claude Code's own auto-compaction is the only cap**, at about 166-177k (window minus the 33k buffer, as section 4 guessed). Each turn paid the 5x rate on 3 requests.
- The drop 134k -> 86-91k before any boundary is unexplained (tool-result clearing?). Unverified.
- The end-of-turn `context_tokens` (65220 on the JSON run) hides the 170k peak. A per-turn peak is not recorded.

Searcher (2026-11-27, current lines): streaming gate `cmd/harnez/agent_run.go:769` (start) and `:1215` (resume); `Observe` at `:806` and `:1297`; detached child gets `--json` at `:727` and `:1233`, plus a spawn at `cmd/harnez/agent_async.go:304` (caller unverified); the parent reads only log files while a child runs (`agent_async.go:174`); `[check:]`/`[done:]` only through `turnStream` (`cmd/harnez/agent_stream.go`); `TokenWatchdog.Observe` reads `Event.ContextTokens` (`internal/subagent/watchdog.go:53-61`); closest test `cmd/harnez/agent_quota_lifecycle_test.go:36-80` (fake driver; whether it reflects the real Claude stream is unverified).

## Milestones

- **M1 (watchdog fires on a real Claude stream):** find why `Observe` never crossed 100k on the canary's streaming turn (is `Event.ContextTokens` set from each `assistant` message's usage, input + cache read + cache write, or only at `result`?). Reproduction test first: a fixture built from the real `claude -p --output-format stream-json` event shapes, with per-message usage crossing the threshold, must stop the turn. Report what happens after the stop (error, compaction, resume) and whether work is lost.
- **M2 (same watchdog for `--json` and detached turns):** `--json` start and resume, and the detached child, run the stream internally and arm the watchdog; stdout stays one JSON document; `[check:]` and the stop message go to stderr. Record the per-turn peak prompt in the JSON output and `[done:]` line (`peak_context_tokens` or similar).
- **M3 (docs):** `docs/AutoCompaction.md:13-15` and `docs/HarnezAgentArchitecture.md` describe the real behaviour; the host reruns the canary for both modes.

New spec values, if any, go in `spec/agent.yaml` with schema (`docs/Spec.md`).

## Decision (host, decide mode, M1 plan review 2026-11-27)

Developer plan (dev-1264, haiku:high): the watchdog is never armed for Claude because `subagent.DriverFor` returns the batch `ClaudeDriver` (`internal/subagent/driver.go:44-45`) and only the tool-call guard upgrades it to the stream driver (`cmd/harnez/agent_run.go:761-762`); the parser already sums input + cache read + cache write per assistant line (`internal/subagent/claude.go:387-388`).

- **Canary args:** no `--max-tool-calls`, no tool-call limit. The root cause stands.
- **Arm the watchdog** whenever the driver can stream and the threshold is > 0, independent of `--json` and tool limits; TUI output stays streaming-only.
- **Stop, do not let the turn finish:** each later request in the turn pays 5x. Persist the observed count as the session's `ContextTokens` so the next resume's pre-prompt gate compacts.
- **Stop mechanism:** keep the existing process-group kill in M1. The M3 canary checks that a resume after the stop works (dangling `tool_use`); if it does not, SIGINT plus `auto_compact.watchdog_grace_ms` becomes pre-work.
- **`--json` stop output:** one JSON document with status `stopped` on stdout, the message on stderr, non-zero exit.
- **No auto-resume** after a stop; the caller (host) resumes.
- **Fixture provenance:** the developer may capture one live `claude -p --output-format stream-json --verbose` run with a tiny prompt (stdout only, never `~/.claude`) and base the fixture on it.
- **Peak field:** `peak_context_tokens` on the turn record and JSON output, shown in `[done:]` (M2).
