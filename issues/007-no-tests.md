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

**Config/repo consistency lint:** — RESOLVED, see below.

## Update (2026-09-02)

`make lint` (commit `311226d`, `scripts/lint.sh`) now checks every `commands/*.md` is
registered in `config.yaml`, closing the "Config/repo consistency lint" gap. The pure-helper
coverage gaps above (`sectionBounds`, `stripComments`, `mergeLangs`, `mergePermissions`,
`sanitizeContent`/`stripMetaCommentary`, error paths) are still untested — status remains
Open, scope narrowed to just those items.
