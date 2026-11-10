## Verdict
done: the `visLen` function uses `runewidth` and `truncateVisible` respects visual width, and the empty table condition was adjusted.

## Changes
internal/usage/watch_layout.go
internal/usage/watch_frame.go
internal/usage/watch_layout_test.go
internal/usage/watch_test.go

## Tests
```
go test ./internal/usage/... -run TestTruncateVisible\|TestVisLen
ok  	ubunatic.com/harnez/internal/usage	0.032s
go test ./internal/usage/... -run TestEmptyTableShowsNoAgentUsage
ok  	ubunatic.com/harnez/internal/usage	0.028s
make test
```
Result: tests passed.
