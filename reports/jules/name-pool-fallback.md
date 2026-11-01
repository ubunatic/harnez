## Verdict
Works, automatic naming falls back to suffixing numeric values when the base pool is exhausted, resolving the error while still allowing finished sessions to hold names.

## Approach
Modified `GenerateSessionName` in `internal/subagent/name.go` to add a fallback loop that appends an integer suffix (e.g. `1`, `2`, ...) to the generated name if all adjectives/animals combinations are taken. The names are populated by reading all existing `.json` files in the store directory, meaning both active and finished (but not deleted) sessions continue to hold their names, allowing them to be resumed by name. Releasing names for finished sessions could be done by only keeping the names of active sessions in the registry memory, or marking inactive sessions in a different way, but that would make resuming finished sessions by name harder.

## Tests
- `go test ./internal/subagent/ -run TestGenerateSessionName`: `ok ubunatic.com/harnez/internal/subagent 0.063s`

## Risks
It does not address clearing names for finished sessions, leading to an ever-growing list of names if they are never explicitly cleaned up. The infinite loop could lock if the user has created so many sessions that max int is reached, although highly unlikely.
