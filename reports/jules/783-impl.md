## Verdict
done, a shared helper `procs.IsAlive` correctly filters out processes in zombie or dead state (Z or X) when `syscall.Kill` returns no error.

## Changes
internal/procs/alive.go
internal/procs/alive_test.go
internal/exec/exec_test.go
internal/sound/sound_test.go

## Tests
`go test -v -run "TestRunExecWrapper_TimeoutKillsProcessGroupDescendant|TestFallbackChainTimeoutKillsProcessGroup" ./internal/exec/ ./internal/sound/` - passes with both test cases correctly verifying process death on timeout using procs.IsAlive instead of syscall.Kill.
`go test -v ./internal/procs/ -run TestIsAlive` - passes with IsAlive returning true for the test process itself.
