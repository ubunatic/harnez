# 1268 — headless Claude workers load user settings, hooks and MCP by default; decide and document the context default

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Docs
**Related**: 1076 (closed), 1027, 1057 (closed), 1068

---

## 1. Problem & Motivation

With no `--context-mode`, `harnez agent start` for a Claude worker passes only `--dangerously-skip-permissions` (plus model and output flags). Claude then loads its default settings sources, hooks and MCP servers. The hub-run report asked whether this is intended and whether it is documented. Source: the hub-run report (orchestrator, 2026-11-22).

## 2. Technical Specification / Findings

- `internal/subagent/claude.go:57-66` (`claudeRunArgs`) appends `claudeContextModeArgs(mode)`.
- `internal/subagent/claude.go:67-70`: an empty mode or `default` returns no extra args. `no-skills` and `plain` add `--disable-slash-commands`, `--strict-mcp-config`, and for `plain` also `--setting-sources ""` (`claude.go:76-84`).
- `spec/agent.yaml:55-60`: `context_modes.default` is documented as "all discovered skills, context files, MCP servers, and default tools" (`skills: true`, `mcp: true`). The start path defaults to it.
- `claude --help` (installed CLI) confirms `--setting-sources` (user, project, local) and `--strict-mcp-config`. Whether hooks load under the default is not verified by a run here.
- Tests pin the current argument lists: `internal/subagent/claude_test.go:167-170`, `internal/subagent/driver_test.go:710-711`.
- The hub's claim is confirmed in code. Its design intent is confirmed by the spec's `default` description.

## 3. Implementation & Verification Plan

Decision (recorded here): **intended**. The spec defines `default` as full discovery, and the default start path uses it. No code change now. The architecture doc's Claude row (`docs/HarnezAgentArchitecture.md`, around line 461) is updated in the same commit to say that no context mode means no restriction flags.

Open for the owner: whether headless workers should default to `no-skills`. That would change `spec/agent.yaml` defaults and the pinned test arguments in `claude_test.go`, and it is a separate change. Verification for the doc change: read the updated row against `internal/subagent/claude.go:67-70`.
