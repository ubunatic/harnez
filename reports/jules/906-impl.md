## Verdict
done, fixed `issues new` to skip printing 'local mode' during initial tracker adoption and updated the test assertion to enforce this behavior.

## Changes
- `cmd/harnez/issues.go`: Added conditional `dirExists` check before printing 'local mode'.
- `cmd/harnez/issues_test.go`: Modified `TestRunIssuesNewAdoptsOnlyNewIssuesDirectory` to assert 'local mode' is not printed on fresh repo.

## Tests
- `bash -c 'go test ./cmd/harnez/... -run TestRunIssuesNewAdoptsOnlyNewIssuesDirectory'`: `ok  	ubunatic.com/harnez/cmd/harnez	0.259s`
- `bash -c 'make check'`: Passed all checks (gofmt, go vet, and go test for all packages).
