# 266 — Support Antigravity Session and Agent ID Resolution in Telemetry

**Status**: Open — filed via /issue
**Priority**: P2 (Medium)
**Severity**: Minor (telemetry attribution fix: links agy tool calls and sessions accurately in tool_catalog.sqlite)
**Category**: Telemetry / Multi-Harness Parity
**Related**: [[121-multi-repo-session-and-ticket-id-resolution]], [[117-harnez-rate-command]], [[118-harnez-exec-shell-interceptor]], [[196-agy-native-hooks-plan-alongside-claude-hooks]], `internal/resolve/resolve.go`, `cmd/harnez/rate.go`

---

## 1. Problem & Motivation

`harnez exec` and `harnez rate` automatically resolve the active `session_id` and `agent_id` from process environment variables.

In issue [[121]], `ANTIGRAVITY_SESSION_ID` was included as an unconfirmed placeholder name. In live inspection of the Antigravity runtime, the actual environment variables exported by the runner are:
- `ANTIGRAVITY_CONVERSATION_ID` (e.g. `8dfe521d-1918-497d-a548-fb2394b49f53`, matching the conversation UUID)
- `ANTIGRAVITY_AGENT=1`
- `ANTIGRAVITY_AGENTAPI_EXE`

Because `ANTIGRAVITY_SESSION_ID` was not set:
1. `resolve.Session` fell back to PPID-derived sliding-window lockfiles instead of using the canonical conversation ID.
2. `detectAgent` fell back to `"unknown"`, obscuring agy tool call volume, execution duration, and failure rates in `harnez stats`.
3. Heartbeat commands (`harnez rate --ok`) were not attributed to `agy`.

---

## 2. Technical Design & Architecture

### 2.1 Session ID Resolution Chain
In `internal/resolve/resolve.go`:
- Update `SessionEnvVars` to include `ANTIGRAVITY_CONVERSATION_ID` (placed ahead of placeholder `ANTIGRAVITY_SESSION_ID`).
```go
var SessionEnvVars = []string{
    "CLAUDE_CODE_SESSION_ID",
    "CLAUDE_SESSION_ID",
    "ANTIGRAVITY_CONVERSATION_ID", // confirmed: Antigravity CLI / IDE runtime
    "ANTIGRAVITY_SESSION_ID",      // compatibility fallback
    "CODEX_SESSION_ID",
}
```

### 2.2 Agent ID Detection
In `cmd/harnez/rate.go`:
- Update `rateAgentEnvVars` to map `ANTIGRAVITY_CONVERSATION_ID`, `ANTIGRAVITY_AGENT`, and `ANTIGRAVITY_AGENTAPI_EXE` to agent name `"agy"`.
```go
var rateAgentEnvVars = []struct {
    Env   string
    Agent string
}{
    {"CLAUDE_CODE_SESSION_ID", "claude"},
    {"CLAUDE_SESSION_ID", "claude"},
    {"ANTIGRAVITY_CONVERSATION_ID", "agy"},
    {"ANTIGRAVITY_AGENT", "agy"},
    {"ANTIGRAVITY_AGENTAPI_EXE", "agy"},
    {"ANTIGRAVITY_SESSION_ID", "agy"},
    {"CODEX_SESSION_ID", "codex"},
}
```

### 2.3 Sprint Teardown Heartbeat Integration
In `fresh-sprint` teardown instructions and practices (`docs/practices/AgenticLoop.md` / `SKILL.md`), record a clean `harnez rate --ok` heartbeat at sprint completion to register successful tool call verification.

---

## 3. Scope of Implementation

1. **`internal/resolve/resolve.go` & `internal/resolve/resolve_test.go`**:
   - Add `ANTIGRAVITY_CONVERSATION_ID` to session resolution list and unit tests.
2. **`cmd/harnez/rate.go` & `cmd/harnez/rate_test.go`**:
   - Add Antigravity env detection to `rateAgentEnvVars` and unit tests for `detectAgent`.
3. **`docs/practices/AgenticLoop.md`**:
   - Note sprint teardown `--ok` heartbeat call.

---

## 4. Acceptance Criteria

- [ ] `resolve.Session` resolves `ANTIGRAVITY_CONVERSATION_ID` to the exact session ID string.
- [ ] `detectAgent` identifies `ANTIGRAVITY_CONVERSATION_ID` / `ANTIGRAVITY_AGENT` as `"agy"`.
- [ ] `harnez exec` tool executions in agy write telemetry rows with `agent_id="agy"` and `session_id=<conversation_id>`.
- [ ] `harnez stats` reports `agy` calls in the `AGENT` breakdown table.
- [ ] All tests pass (`make check`).

---

## 5. Verification

- **Automated**: `go test -v ./internal/resolve/...` and `go test -v ./cmd/harnez/...`.
- **Manual Verification**: Run a test command with `ANTIGRAVITY_CONVERSATION_ID=test-conv-123 ANTIGRAVITY_AGENT=1 harnez rate --ok "test"` and verify `harnez stats` attributes to `agy`.

