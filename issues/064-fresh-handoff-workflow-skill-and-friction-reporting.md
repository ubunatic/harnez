# 064 — Lean Fresh-Handoff Workflow Skill and Calibrated Friction Reporting

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics & Workflow Skills
**Related**: [[039-agentic-loop-practices-and-sprint-command]], [[040-agent-context-duplication-and-file-read-discipline]], `docs/AgenticLoop.md`

---

## 1. Problem & Motivation

The standard 5-phase agentic sprint loop (`docs/AgenticLoop.md`: Advisory → Dev → Review → Hygiene → Retro) provides rigorous quality gates for major architectural changes. However, for focused, day-to-day pairing tasks, running the full 5-phase loop with dedicated reviewer subagents introduces unnecessary latency and token overhead.

Real-world experience in pairing across sibling projects (such as `lmcoder`) demonstrates that:
1. **Fresh Agents Prevent Context Bleed**: Disagreeable bugs, stale assumptions, and token bloat are drastically reduced when the orchestrator dispatches a fresh subagent with a single, clear goal rather than carrying an oversized, multi-turn session.
2. **Confidence-Gated Reviews Save Compute**: When a dev subagent achieves 100% automated test verification (`go test ./...`, `go vet ./...`) with high confidence, the primary orchestrator can perform a rapid inline review without spinning up an expensive separate reviewer agent.
3. **Friction Reporting Needs Calibration**: Subagents should capture genuine environment papercuts or harness quirks, but must **not over-report** or repeatedly complain about known noise on fast iterations. Friction reporting is most valuable after substantive work sessions.
4. **Trust the Base Framework**: Orchestrators should not micromanage or duplicate standard workspace/tooling conventions already present in the agent's base system prompt.

---

## 2. Technical Specification

### 2.1 The Fresh-Handoff Pattern (`fresh-sprint` / `fresh-handoff`)

Define a lightweight workflow skill in `harnez` with the following principles:

1. **Clean Goal Handoff**:
   - The orchestrator provides a concise problem statement, relevant issue/spec references, and explicit verification targets.
   - Trusts the subagent's framework/system prompt for standard tool usage and code conventions without redundant preamble.
2. **Autonomous Execution & Self-Verification**:
   - The dev subagent implements the fix/feature and executes repo-native verification commands (`go test ./...`, `make check`, canary probes).
3. **Confidence-Gated Inline Review**:
   - If tests pass cleanly and confidence is high, the dev agent commits and returns a concise summary directly to the orchestrator.
   - Bypasses formal reviewer subagent dispatch unless the change has high cross-project blast radius or ambiguity.
4. **Calibrated Friction Reporting**:
   - The subagent reports authentic tool, environment, or sandbox friction **only** when genuine hurdles were encountered during non-trivial sessions.
   - Avoids recurring boilerplate or over-reporting on routine, fast iterations.

### 2.2 Integration in `harnez`

- Add `/fresh-sprint` or `/fresh-handoff` slash command and skill description in `harnez`.
- Update `docs/AgenticLoop.md` with guidance on when to choose the Lean Fresh-Handoff vs. the Formal 5-Phase Loop.

---

## 3. Implementation & Verification Plan

1. Document the Fresh-Handoff skill in `harnez` skills registry (`commands/` or `contrib/skills/`).
2. Add section in `docs/AgenticLoop.md` defining the Fast-Path / Fresh-Handoff tier alongside the standard 5-phase loop.
3. Verify formatting and linting via `harnez status`.
