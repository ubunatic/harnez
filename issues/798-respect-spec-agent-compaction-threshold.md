# 798 — respect spec: agent compaction threshold

**Status**: Closed — spec/agent.yaml compact_threshold_tokens + schema, constant removed (f49bb0df)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Spec

---

## Problem

The default context token threshold for automatic agent compaction is a
hardcoded application behavior value. Provider/model-specific user overrides
are read from local configuration, but the embedded agent spec does not
currently own the default used when those overrides are absent.

## Candidates

| Location | Value | Target spec file and key |
|---|---|---|
| `internal/subagent/compact.go:12` | 200,000 context tokens | `spec/agent.yaml: compact_threshold_tokens` |

## What stays in code

Provider/model override lookup and the positive-threshold validation stay in
code because they implement user-local override behavior and input validation.
The threshold comparison and verified-compaction checks are algorithm and
provider-response mechanics.

## Acceptance criteria

- Declare `compact_threshold_tokens` in `spec/agent.yaml` and its JSON schema.
- Code reads the default from the embedded agent spec, with no mirrored
  fallback constant.
- Add a test that loads the embedded spec and asserts the key exists and is
  consumed.
