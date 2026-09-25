# 584 — mcp: tools/list emits required:null, Claude Code rejects it

**Status**: Closed — Omitted nil required fields from all MCP tool schemas and added tools/list array validation; make test-q1 passed.
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: `internal/mcp`

## 1. Problem & Motivation

Claude Code rejects the MCP server's `tools/list` response because tool schemas can
serialize `required` as `null` when the Go slice is nil. This affects
`harnez_list_agents` and may affect any tool without required parameters.

## 2. Goal & Acceptance Criteria

- No tool schema serializes `required: null`; the field is omitted or an array.
- A server test inspects every tool returned by `tools/list` and checks `required`
  whenever present is a JSON array.
- `make test-q1` passes.
