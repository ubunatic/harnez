# 691 — track Claude and Agy sessions in agent list

**Status**: Open — provider discovery work remains
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [External agent session discovery](../docs/ExternalAgentSessions.md), [684 — agent list discovers externally hosted sessions](684-agent-list-discovers-externally-hosted-sessions.md)

---

## 1. Problem & Motivation

`harnez agent list` currently discovers active Codex sessions that were started
outside Harnez. Claude Code and Agy sessions are still missing, so the command
does not yet provide one view of active sessions across the providers the user
runs.

## 2. Technical Specification / Findings

- Investigate provider-owned local session metadata and live-process matching
  for Claude Code and Agy.
- Keep discovery read-only and metadata-bounded. Do not parse conversation
  contents or infer that an old session is active based only on its existence.
- Define provider paths, formats, matching windows, and field mappings in
  `spec/agent.yaml` with schema validation.
- Reuse the active-by-default behavior and explicit historical-session filters
  from Codex discovery.
- Show context capacity, current context use, and session-wide used tokens only
  where each provider exposes reliable values; otherwise mark those values
  unknown.

## 3. Implementation & Verification Plan

**Goal**: Include active externally managed Claude Code and Agy sessions in
`harnez agent list` with stable provider identities and accurate available
usage metadata.

Done when active sessions for both providers are discovered without duplicating
Harnez-managed entries; inactive sessions remain hidden by default and can be
listed with the historical-session option; session metadata and supported usage
metrics are documented; and parsing, process matching, and deduplication are
covered by focused tests. If a provider has no safe discovery mechanism, record
the limitation and leave that provider unsupported.
