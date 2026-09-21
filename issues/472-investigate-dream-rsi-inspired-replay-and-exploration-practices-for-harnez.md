# 472 — Investigate Dream-RSI-inspired replay and exploration practices for harnez

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [AgenticLoop practice](../docs/practices/AgenticLoop.md), [Dream-RSI paper](https://arxiv.org/abs/2609.14858), [Dream-RSI project](https://dream-rsi.com/)

---

## 1. Problem & Motivation

Dream-RSI improves an agent's exploration policy without changing the underlying model: it replays recorded discovery runs as a simulator, evaluates alternative policies cheaply, and deploys a better policy for the next round. Harnez already records issue work, agentic-loop activity, telemetry, and retrospectives, but does not clearly turn that history into replayable strategy experiments or reusable workflow improvements.

## 2. Technical Specification / Findings

/goal: Determine whether a safe, useful subset of Dream-RSI can be adopted in harnez, and document a concrete recommendation for new practices and/or commands.

Investigate:

- Which existing artifacts can serve as replayable trajectories: issue histories, advisor findings, tool feedback, test outcomes, telemetry, and retrospectives.
- Whether a deterministic or sanitized replay format can evaluate alternative prompts, phase ordering, delegation policies, or verification gates without mutating repositories or contacting external systems.
- Candidate practice documentation, such as a bounded exploration-policy feedback loop with explicit human approval and regression safeguards.
- Candidate commands (for example, replay/evaluate/suggest) and their fit with `apply` versus `init`, current telemetry, quota-1 guardrails, and process-hygiene rules.
- Hard limits and risks: incomplete histories cannot generate unseen outcomes; noisy or sensitive logs may bias evaluation or leak data; policy changes must not silently self-modify production behavior.

## 3. Implementation & Verification Plan

Produce a short design note or issue follow-up that:

1. Maps Dream-RSI concepts to harnez components and identifies the smallest viable canary.
2. Recommends adopt, defer, or reject for each proposed practice/command, including boundaries and privacy/safety controls.
3. If adoption is justified, files or links the narrowly scoped implementation tickets and defines replay/evaluation acceptance criteria.
