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

---

## Implementation Plan

### Current state of the named gaps (verified 2026-09-04)

| Ticket name | Where it lives now | Test status |
|---|---|---|
| `sectionBounds` | `internal/markdown/markdown.go:95` `SectionBounds` (exported), plus `locateSection` (:109) | `markdown_test.go` covers markers + Diff only; no bounds edge cases |
| `stripComments` | `internal/jsonc/jsonc.go:47` `StripComments` | untested; `/* */` still unhandled |
| `mergeLangs` | **gone** — no such symbol remains | n/a, drop from scope |
| `mergePermissions` / `applyMerge` | `internal/claude/apply.go:63` / `:74` | untested (see issue 005, which rewrites both) |
| `sanitizeContent` / `stripMetaCommentary` | `internal/claude/init.go:36` / `:58` | untested |
| error paths | `LoadConfig`, `fsutil.ExpandHome`, unknown `--docs` name | untested |

The ticket's "Config/repo consistency lint" item is already resolved (`scripts/lint.sh`).

### Steps — one test file per package, in this order

1. **`internal/markdown/markdown_test.go`** (extend) — `TestSectionBounds` table:
   begin without end; end without begin; end before begin; two nested/overlapping section
   pairs; markers inside a fenced code block; CRLF line endings; section named as a prefix of
   another (`Foo` vs `Foo Bar`). Assert `found` and the exact line indices.

2. **`internal/jsonc/jsonc_test.go`** (extend) — `TestStripComments` table:
   `//` at line start / after a value / trailing without newline; `//` inside a string
   (`"http://x"` — the current escape-aware loop should preserve this, assert it);
   escaped quote before a comment; `/* block */` inline and multi-line.
   **`/* */` is genuinely unimplemented** — write the test as the spec, then add block-comment
   handling to `StripComments` (a second state flag alongside `inString`). This is the one
   place in this ticket where a real code change is warranted, since JSONC files written by
   hand legitimately contain block comments and `jsonc.Read` currently returns an empty map
   for them, which `hasSettingsKey`/`applyMerge` then silently read as "nothing applied".
   Add `TestRead_BlockCommentedFile` at the `Read` level to lock that in.

3. **`internal/claude/init_test.go`** (extend) — `TestSanitizeContent` table: fenced output
   with and without a language tag; fence without a closing fence (must not truncate);
   `<!-- harnez: -->` and `<!-- claudeconfig: -->` marker lines dropped, other HTML comments
   kept; leading/trailing whitespace. `TestStripMetaCommentary` table: no `---` (identity);
   long body + short trailing note → body kept; short preamble + long body → body kept;
   both long → identity (ambiguous, must not guess); several `---` rules (only the last is
   considered). Empty string for both.

4. **`internal/claude/apply_test.go`** (new) — `TestMergePermissions` / `TestApplyMerge`:
   union dedup and order stability; non-`permissions` keys replaced not merged; `permissions`
   present in incoming but not existing (and vice versa); non-map `permissions` value falls
   back to replace without panicking.
   **Coordinate with issue 005**, which changes both signatures — write these tests as part
   of 005 if 005 lands first, otherwise 005 inherits and extends them.

5. **Error paths** — `TestLoadConfig_Errors` (missing file, malformed YAML, unknown language
   name in `--docs`) and one unwritable-target case in `internal/claude` using a `0555`
   temp dir (`t.Skip` when running as root, since root ignores the mode).

### Design decisions

- **Table tests, no new helpers or fixtures framework.** These are pure functions; a
  `[]struct{name, in, want string}` per function is the whole apparatus.
- **Assert exact outputs, not "no error".** The review gate in this repo explicitly checks
  for assertion rigor; `err == nil` alone would be a rubber stamp.
- **Scope stays at pure helpers.** End-to-end apply/init/status behaviour is already covered
  by `integration_test.go`; do not duplicate it here.

### Risks / open questions

- Adding `/* */` support to `StripComments` is behaviour change, not just a test — it is
  strictly more permissive (files that previously failed to parse now parse), so the risk is
  low, but it deserves its own commit separate from the pure test additions.
- Overlap with issue 005 on `mergePermissions`; sequence them rather than running both in
  parallel to avoid conflicting edits to `apply.go`.

### Scope

**Medium** — mostly mechanical; ~400 lines of tests across four files, plus ~30 LOC of
block-comment support in `jsonc`. Splittable into per-package commits.
