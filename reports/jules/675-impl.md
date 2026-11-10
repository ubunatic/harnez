## Verdict
done, `driver.Compact` now takes and passes the session's `Model`, and `claudeModelArgs` preserves it correctly to avoid defaulting to a larger model.

## Changes
- `internal/subagent/driver.go`
- `internal/subagent/agy.go`
- `internal/subagent/claude.go`
- `internal/subagent/codex.go`
- `internal/subagent/jules.go`
- `internal/subagent/pi.go`
- `internal/subagent/agy_test.go`
- `internal/subagent/claude_test.go`
- `internal/subagent/driver_test.go`
- `internal/subagent/pi_test.go`
- `cmd/harnez/agent_run.go`
- `cmd/harnez/agent_args_test.go`
- `cmd/harnez/agent_delete_test.go`
- `cmd/harnez/agent_models_set_test.go`
- `cmd/harnez/agent_resume_test.go`
- `cmd/harnez/agent_root_test.go`
- `cmd/harnez/agent_spec_test.go`

## Tests
- `gofmt -w internal/subagent/driver_test.go` (success)
- `make test` (success)
