## Verdict
done: I added a `setupFakeMinisign` helper to create a fake `minisign` on PATH and invoked it in the five failing tests.

## Changes
`internal/release/release_test.go`

## Tests
`go test ./internal/release/... -run "TestDryRunRelease|TestDryRunReleaseContinue|TestPreflightForgeValidation|TestTagPrefixFormatting|TestBuildCmdMakefileFallback"`
`ok  	ubunatic.com/harnez/internal/release	0.145s`
