# No test coverage

**Severity:** Low for now, blocks confidence for wider adoption

## Problem

`go test ./...` reports `[no test files]`. The smoke test (`scripts/smoke-test.sh`) covers
the happy path (apply → idempotency → drift → repair) against the live `~/.claude` directory
but does not cover:

- Edge cases in `sectionBounds` (malformed markers, overlapping sections)
- `stripComments` JSONC parsing (especially with `/* */` block comments, which are not handled)
- `mergeLangs` dedup (the existing bug in issue #003 would be caught by a simple table test)
- `mergePermissions` / `applyMerge` union logic
- `sanitizeContent` / `stripMetaCommentary` LLM output cleaning
- Error paths (missing home dir, unwritable target, unknown language)

## Suggested starting point

Unit tests for the pure functions are straightforward:
- `sectionBounds`, `applySectionMD` (in-memory string operations)
- `unionStrings`, `mergeLangs`, `mergePermissions`
- `stripComments`, `sanitizeContent`, `stripMetaCommentary`
- `buildSettingsDoc`, `resolveModel`

Integration tests for `applyAll`/`diffAll`/`cleanAll` can use `os.MkdirTemp` to isolate
from the real `~/.claude`.
