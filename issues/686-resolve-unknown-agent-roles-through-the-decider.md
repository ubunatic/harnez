# 686 — Resolve unknown agent roles through the decider

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**:

---

## 1. Problem & Motivation
`harnez agent --role <name>` rejects names that do not exactly match a
configured role. Users may describe an intended role with different wording,
so the command should use the Harnez decider (Jev) to map an unknown name to a
known role when the decider is available.

## 2. Technical Specification / Findings
Keep the existing known roles as the valid outputs. When Jev is unavailable,
or cannot confidently map the name, report the unknown role and list the
available roles rather than starting with an unverified role.

## 3. Implementation & Verification Plan
**Goal**: Let `harnez agent --role <name>` resolve an unfamiliar role name to
an existing configured role through Jev when available. Done when a confident
mapping starts with the mapped role, while unavailable or uncertain decisions
fail clearly and preserve the known-role list. Stop and report if the decider
cannot safely distinguish a mapping from an unknown role.
