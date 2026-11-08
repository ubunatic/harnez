## Verdict
done: I modified `StartRole` in `internal/agent/core.go` to return an `inferred` boolean and updated `WithProtocol` to include a warning when `inferred` is true, then updated all call sites appropriately.

## Changes
- `internal/agent/core.go`
- `internal/agent/core_test.go`
- `cmd/harnez/agent_run.go`
- `cmd/harnez/agent_interactive.go`
- `cmd/harnez/agent_async.go`
- `cmd/harnez/agent_stream.go`
- `cmd/harnez/agent_stream_test.go`
- `cmd/harnez/agent_role_test.go`
- `cmd/harnez/agent_caller_test.go`

## Tests
- `go test ./internal/agent/`: ok
- `go test -run TestAgentRole ubunatic.com/harnez/cmd/harnez`: ok
- `go test ./...`: The test `TestInitInstallsIssueMergeDriverEndToEnd` failed with `harnez: not found`, and some `release` tests failed due to `minisign` missing, which are both unrelated to my changes. All `agent` related tests passed successfully.
