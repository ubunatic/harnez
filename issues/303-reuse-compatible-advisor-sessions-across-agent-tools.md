# 303 — Reuse compatible advisor sessions across agent tools

**Status**: Closed — implemented in 327753e: prose-first harnez-advisor skill with four-target distribution test and native-resume guidance
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [Plugin-system assessment](../docs/studies/2026-09-10-agent-harness-plugin-systems-and-self-modification.md), [Advisor session reuse study](../docs/studies/2026-09-10-advisor-session-reuse-and-cross-agent-dispatch.md), commit `f1ac8ef`

---

## 1. Problem & Motivation

Harnez sometimes starts a new advisor session even though a compatible prior session could be resumed. Reusing the previous Astra low-reasoning session `01a08a2e-99b6-7e42-9215-d38eb4f38546` for the Harnez plugin-system assessment preserved the advisor's existing context and produced the assessment committed in `f1ac8ef`. The project currently has no documented way to determine whether that reuse saved input tokens, latency, or cost, and no common workflow for applying the strategy across supported agent tools.

Design and document a Harnez command or skill that prefers safe reuse of compatible advisor sessions, defaults to low reasoning when appropriate, exposes evidence about the resulting savings, and falls back to a fresh session when reuse is unavailable or unsafe. The design must cover Claude, Codex, Gemini/AGY, and Prime wherever each tool supports session resumption.

## 2. Technical Specification / Findings

The investigation must answer whether the current session exposes cache, usage, or cost statistics that can compare the resumed Astra run with a fresh-session baseline. It must inspect available local metadata, tool output, and provider APIs or CLIs where accessible, and record an explicit observability gap when those statistics are unavailable. It must not infer token savings solely from a session ID or from the fact that context was resumed.

The proposed interface should define:

- discovery and validation of candidate prior advisor sessions;
- a compatibility identity covering agent tool, model, task or project scope, relevant instructions, permissions, and session age or validity;
- per-tool capability differences for Claude, Codex, Gemini/AGY, and Prime, including whether resumption, cached context, usage reporting, and low-reasoning controls exist;
- a low-reasoning default with an explicit way to request a different reasoning level;
- safe fallback to a fresh session when identity, capability, validity, or observability checks fail;
- evidence output that distinguishes measured savings, provider-reported usage, estimated savings, and unknown values;
- safeguards against reusing context across unrelated tasks, projects, identities, permissions, or incompatible model/tool versions.

The design should preserve a durable record of the selected session, compatibility decision, fallback reason, reasoning setting, and available usage evidence so a later review can reproduce the decision without exposing secrets or full private transcripts.

## 3. Implementation & Verification Plan

1. Inventory the current Harnez agent integrations and the available session, cache, usage, and cost metadata for each supported tool.
2. Analyze the Astra session identified above and report whether this current environment can establish token or cost savings; state precisely which measurements are absent.
3. Specify the command or skill contract, compatibility rules, per-tool capability matrix, output schema, and fresh-session fallback behavior.
4. Add focused tests or deterministic fixtures for compatible reuse, incompatible sessions, expired or missing sessions, unsupported resumption, low-reasoning selection, and measured versus unknown savings.
5. Verify that the workflow never reuses a session solely because it is recent or has a matching provider, and that its evidence output does not claim savings without provider or local measurements.

Acceptance criteria:

- A self-contained design exists for one Harnez command or skill covering Claude, Codex, Gemini/AGY, and Prime capability differences.
- The Astra example and commit `f1ac8ef` are used as traceable input to the investigation.
- The implementation explicitly reports whether current cache, usage, or cost statistics can prove savings and documents any observability gap.
- Session compatibility, task isolation, low-reasoning defaults, fallback to a fresh session, and unsafe-reuse rejection are specified and testable.
- Verification guidance covers both supported resumption and tools that lack resumable sessions or reliable usage reporting.

## 4. Uncertainties and Open Questions

- Which agent providers expose cache-read, cache-write, input-token, output-token, latency, and cost fields for resumed sessions?
- Does a resumed session's provider accounting distinguish reused context from newly billed input, or can Harnez only report an estimate?
- What stable identifiers and task metadata are available from each tool, and how long do their sessions remain valid?
- Should the command choose a candidate automatically, or require an explicit session selector when compatibility is ambiguous?
- What is the minimum metadata that can be persisted safely without retaining sensitive advisor content?
