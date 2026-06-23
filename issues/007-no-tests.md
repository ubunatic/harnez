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

## Config/repo consistency lint

Every `commands/*.md` file in the repo should have a matching entry in `config.yaml`.
Currently nothing enforces this — a file can be added (or removed) without updating
`config.yaml` and apply silently ignores it (discovered 2026-06-23: `commands/evergreen.md`
existed in the repo from commit `a4f2467` but was never installed).

Add a lint check that diffs `ls commands/*.md` against the `commands:` entries in
`config.yaml`. Could be a `make lint` target, a Go test, or a check in `smoke-test.sh`.
