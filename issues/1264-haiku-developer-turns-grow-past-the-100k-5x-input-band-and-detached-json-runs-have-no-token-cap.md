# 1264 — Haiku developer turns grow past the 100k 5x input band, and detached --json runs have no token cap

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 591, 592 (closed), 969, 1033, 956, 973; study `docs/studies/2026-11-21-headless-signal-assistant-one-day-build-story.md` (table row 9)

---

## 1. Problem & Motivation

Haiku 5.5 input above 100k tokens per request costs 5x (`docs/AutoCompaction.md:94`, citing the Anthropic pricing page; `spec/agent.yaml:224`). In the 2026-11-21 headless run, Haiku developer turns ended with context usage above that band: sig-voice 151.0k, sig-voice-fix 145.7k, sig-poll-fix 143.5k, hub-006b 115.0k, sig-poll 112.3k, sig-rich2 112.2k (study table row 9, line 131). Haiku's compaction threshold is 80k (`spec/agent.yaml:31-33`). The study records no cap that stopped these turns.

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

Canary before any build (repo rule: canary first): one detached Haiku turn with a prompt above 100k tokens, to measure whether the 80k cap fires. Cost needs owner approval; not run in this ticket.

## 4. Notes

- **2026-11-22, hub-run (C): the 80k gate does not bound a single turn.** The orchestrator's hub run had Haiku workers reach about 167k context inside one long turn. Harnez's gate runs only before a prompt (`docs/AutoCompaction.md:13-15`), so nothing in harnez stopped the turn. The CLI's own auto-compaction did not hold it under 100k either: `docs/AutoCompaction.md:38` gives the Claude Code autocompact buffer as 33k by default, so the trigger sits near the window size minus 33k. If Haiku's window is 200k (unverified here), that gives about 167k, which matches the report. Source of the 167k figure: the orchestrator's hub log, not in this repo. Verify with the canary in section 3 and read the stream's usage events for the turn.
