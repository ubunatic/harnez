# 452 — Allow harnez agent stop --all and harnez agent delete --all

**Status**: Closed — implemented stop --all and delete --all with lineage-safe behavior and tests
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: —

---

## 1. Problem & Motivation

The agent lifecycle commands do not provide an explicit all-agents operation,
making it cumbersome to stop or delete every managed agent in one command.

## 2. Technical Specification / Findings

Add support for:

- `harnez agent stop --all`
- `harnez agent delete --all`

The commands should apply the existing stop/delete semantics to all eligible
managed agents and handle the empty-agent case cleanly.

## 3. Implementation & Verification Plan

/goal: Users can stop or delete all managed agents with the corresponding
`--all` command, with safe behavior for no matching agents and tests covering
the new flag paths.

- Extend command argument validation and dispatch for `--all`.
- Add or update tests for both commands, including no-agent behavior.
- Run the relevant test suite and verify help text/documentation.
