# 304 — Advisor lifecycle CLI and metadata tracking

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [Issue 303](303-reuse-compatible-advisor-sessions-across-agent-tools.md), [HarnezAdvisor prose-first skill](../docs/commands/HarnezAdvisor.md), commit `327753e`

---

## 1. Problem & Motivation

Issue #303 established the prose-first advisor reuse workflow and was closed in commit `327753e`. The current contract lives in `docs/commands/HarnezAdvisor.md`, but an orchestrator cannot yet query advisor lifecycle state or apply the reuse policy through a stable CLI and metadata model. As a result, last use, project and feature scope, task identity, compatibility, staleness, compaction, native resume, fresh fallback, and token or cache evidence remain implicit in prose and are difficult to audit consistently.

Define and implement the future Harnez advisor lifecycle interface. Example commands include `harnez advisor status`, `harnez advisor compact <session>`, and `harnez advisor reuse --stale-after ...`. The design must support safe orchestration across the agent tools covered by issue #303 while preserving the existing rule that a Harnez telemetry session ID is only a correlation identifier, never a provider resume identifier.

## 2. Technical Specification / Findings

Define a versioned, machine-readable metadata schema and lifecycle state transitions for advisor sessions. The schema should let the orchestrator record, query, and update:

- provider and native session identity, with provider resume data kept in the appropriate local integration rather than confused with Harnez telemetry IDs;
- last-use time, project, feature, task scope, advisor role, model, instruction context, permission context, and compatibility decision;
- validity and staleness state, including the configurable threshold supplied by `--stale-after`;
- compaction requested, compaction completed, and the concise retained-context summary needed before resumption;
- native resume capability and result, or the reason a fresh advisor was selected;
- token, cached-input, output, latency, and cost fields when available, with each value labelled `measured`, `reported`, `estimated`, or `unknown`.

Specify the command behavior and output contract for status, compaction, reuse, and fresh fallback. `reuse` must validate project, feature, task, provider, model, role, instruction, permission, capability, and session validity before resuming. A stale compatible session should be compacted before reuse when the harness supports that operation. Missing, mismatched, unsupported, expired, or ambiguous data must produce an explainable fresh fallback. A recent session alone is never sufficient evidence of compatibility.

Metadata must contain only the minimum durable facts required to reproduce lifecycle decisions. It must never persist credentials, secrets, prompts, arbitrary command output, or full advisor transcripts. Native provider identifiers and other sensitive values must be handled according to the integration's storage boundary and excluded from shared tracker or telemetry records unless explicitly safe.

## 3. Implementation & Verification Plan

1. Inventory the existing advisor integrations and identify the local boundary at which native resume, compaction, and usage data are available.
2. Define the metadata schema, lifecycle transitions, compatibility predicate, staleness policy, and evidence vocabulary without duplicating provider-specific facts that cannot be observed.
3. Implement the CLI commands and an orchestrator-facing output format, including dry-run or inspection behavior where needed to explain reuse and fallback decisions.
4. Add focused tests or deterministic fixtures for compatible reuse, stale-session compaction, native resume, fresh fallback, incompatible scope or permissions, unsupported compaction or resume, and measured versus reported, estimated, and unknown token or cache evidence.
5. Verify that metadata redaction excludes secrets and transcripts and that the commands do not claim token or cache savings without comparable provider or local measurements.

Acceptance criteria:

- `status`, `compact`, and `reuse --stale-after ...` have a documented, versioned contract or an explicitly recorded provider limitation.
- The orchestrator can determine last use, project/feature/task scope, compatibility, staleness, compaction state, resume capability, and fresh-fallback reason from durable metadata.
- Reuse validates all relevant identity and permission context, compacts stale compatible sessions when supported, and falls back to a fresh session with an explicit reason when it cannot safely resume.
- Token and cache evidence distinguishes measured, reported, estimated, and unknown values; no savings claim is made from session reuse alone.
- Persisted metadata contains no secrets or full transcripts, and tests verify the redaction boundary.
- The implementation remains compatible with the prose rules and provider differences documented by issue #303 and `docs/commands/HarnezAdvisor.md`.

## 4. Uncertainties and Open Questions

- Which provider integrations can expose a stable native resume handle and compaction operation without placing credentials or transcript content in shared metadata?
- Should the lifecycle store retain provider-native identifiers directly, or store only an integration-local reference with a resolvable capability check?
- What timestamp and retention policy should apply when a provider does not report last use or session expiry?
- Can cached-input counts be compared with a fresh control closely enough to report measured savings, or must the result remain reported, estimated, or unknown?
- Which lifecycle mutations should be idempotent, and how should concurrent orchestrators serialize updates to the same advisor record?
