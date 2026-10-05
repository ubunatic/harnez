# 712 — Add support for the Pi coding agent CLI 1.0+

**Status**: Closed — Pi 1.0+ driver, retry handling, RPC compaction and live session canary delivered; Q1 suite still fails on pre-existing spec expectation mismatches
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [#291 external coding-agent dispatch](291-add-harnez-agent-command-standardized-non-interactive-dispatch-to-external-coding-agent-clis-codex-agy.md), [#302 Pi and agent harness research](302-research-plugin-systems-for-pi-agents-deepseek-harness-and-prime-agent-self-modification.md), [#070 Pi cross-agent canaries](070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode.md)

---

## 1. Problem & Motivation
Harnez's agent runtime has no first-class driver for the Pi coding agent.
Support the current Pi CLI, focusing on version 1.0 and later, so it can be
selected alongside existing agent backends for coding tasks.

## 2. Technical Specification / Findings
Pi 1.0.3 is installed. Its `pi --help` and CLI documentation confirm:

- `--mode json` runs a non-interactive invocation and emits JSONL events,
  including a session header, completed messages, usage, and `agent_settled`.
- `--model <pattern>` selects a configured Pi model; `--thinking` accepts the
  documented levels.
- `--session <id>` resumes an existing project session. Sessions persist under
  Pi's session store and are scoped by working directory.

Older pre-1.0 behavior is out of scope. Existing Pi canary and extension
research do not provide agent-driver support.

Implementation added `PiDriver` in `internal/subagent/pi.go`, selected by the
Harnez provider registry. `pi` uses Pi's configured default model;
`pi:<provider/model-id>[:low|med|high]` selects another Pi model without a
Harnez-side static catalog. The driver parses completed assistant events and
usage, requires the JSON run to settle, and resumes using Pi's provider session
ID. `docs/PiAgent.md` documents use and lifecycle limitations.

Live canaries against Pi 1.0.3 and a local lmcoder backend verified new turns,
resume, and manual compaction: the session compacted through Pi's RPC command,
then resumed and recalled the canary token `TOPAZ`. Pi's compact response gives
an estimated post-compaction context size, not an exact token count. The
port-8737 backend was stopped; the pre-existing lmcoder proxy remains running.

The `make test-q1` run failed on existing assertions that conflict with the
committed `spec/agent.yaml` state (Codex default model and flash38 guidance).
No test assertions or those spec settings were changed. The Pi tests ran in the
suite. The final RPC stdin-lifecycle change in `internal/subagent/pi.go` was made
after that quota run; it was build- and live-canary-verified but not covered by a
subsequent test-suite run.

## 3. Implementation & Verification Plan
/goal Add and document a Pi 1.0+ driver that supports the Harnez agent task and
session workflow, with command-construction tests and a live canary against a
current Pi release; or stop and report when blocked on user input or denied
permission.
