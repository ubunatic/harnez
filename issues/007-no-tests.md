# Thin test coverage — gaps remaining

**Status:** Open — partially addressed

**Severity:** Low

## What exists

- `internal/claude/integration_test.go` — apply, diff, status, init, clean, evergreen/skill
  install paths (`~/.gemini/skills/`, `~/.codex/skills/`), idempotency
- `go test ./...`, `go vet ./...`, `make install` are part of the verified path

## Remaining gaps

**Pure helper coverage is thin:**
- `sectionBounds` — malformed markers, overlapping sections not exercised
- `stripComments` JSONC parsing — `/* */` block comments are not handled and not tested
- `mergeLangs` dedup — the bug in issue #003 would be caught by a simple table test
- `mergePermissions` / `applyMerge` union logic
- `sanitizeContent` / `stripMetaCommentary` LLM output cleaning
- Error paths: missing home dir, unwritable target, unknown language

**Config/repo consistency lint:**
Every `commands/*.md` in the repo should have a matching entry in `config.yaml`. Nothing
enforces this — a file can be added (or removed) without updating `config.yaml` and apply
silently ignores it. A `make lint` target or Go test diffing `ls commands/*.md` against
`config.yaml` entries would catch this.
