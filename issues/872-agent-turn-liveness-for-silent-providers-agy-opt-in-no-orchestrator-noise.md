# 872 — agent turn liveness for silent providers (agy), opt-in, no orchestrator noise

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 179 (rate --ok heartbeat), 125 (async feedback instead of polling), 306 (Codex quarantine)

---

/goal An orchestrator can tell whether a running `harnez agent` turn is alive for agy, Claude and Codex
developers, from data that is safe to read. The signal is off by default and switchable in the spec or
per run. It adds no messages to the orchestrator beyond the existing heartbeat cadence and writes no
growing logs. The design is evaluated against the current stable agent flow before any code lands;
or stop and report when blocked on a user decision.

## 1. Problem & Motivation

loom 373 (2026-10-23): an `agy:gemini-3.8-flash` developer turn showed only `start` in the job log for
about 3 minutes and was stopped as "stuck". Its agy conversation log showed 42 normal tool steps
(reads, `loom measure`) and model calls every few seconds. A Codex turn in the same sprint printed
`[message]`/`[heartbeat]` lines and looked alive. Silence from agy is a provider difference, not a stall.

## 2. Constraints (user, 2026-10-23)

The agent flow is stable now; changes to orchestrator/subagent flow need careful design and strong
evaluation.

- **Safe sources only:** read-only, documented or stable provider outputs. Do not parse private
  databases or prompts, and never copy conversation content (prompts, file contents, tool output)
  into harnez output. Counts and timestamps only, e.g. "N steps, last activity 4s ago".
- **Cross-agent:** one signal model for agy, Claude and Codex (Codex already streams; reuse it).
- **No noise:** no extra orchestrator messages. At most fold the signal into the existing heartbeat
  line at its current cadence, with no per-step lines.
- **No unbounded logs:** nothing that grows per step; constant-size state or none.
- **Switchable:** on/off in the spec, and a per-run flag; default off until evaluated.

## 3. Findings / open questions

- agy candidates: `~/.gemini/antigravity-cli/cli.log` (one `streamGenerateContent` line per model
  call; shared by all agy sessions, so it needs per-session attribution), or the per-conversation
  `conversations/<id>.db` mtime (mtime only, no content read). Is either stable across agy releases?
- Claude: does the headless stream-json output give step events already?
- Does the existing heartbeat line have a field for "last activity" for all providers?
- Evaluation: replay a canary turn per provider (docs/Canary.md) and measure that the orchestrator
  output and log sizes stay unchanged when the switch is off.

## 4. Verification

- Switch off: byte-identical job output against today for all three providers.
- Switch on: a silent agy turn shows "last activity" in its heartbeat; no new lines between heartbeats;
  no file growth per step.
