# 011 — autoDetectDocs non-deterministic order causes spurious AGENTS.md diffs

**Status:** Closed — fixed in `444c29f` (2026-07-05)  
**Severity:** Low — cosmetic / idempotency, no functional impact

---

## Problem

`autoDetectDocs` in `internal/claude/init.go` originally iterated over `cfg.AgentsMD.Languages`, which is a Go map (`map[string]Language`). Because Go map iteration order is randomized per runtime execution, the order in which auto-detected docs were collected and appended to `detected` was non-deterministic.

When generating the `Language Conventions` section of `AGENTS.md`, this non-deterministic ordering caused `init` to produce spurious diffs between runs even when project configuration and detected languages did not change.

## Resolution

Fixed in commit `444c29f` (2026-07-05):

1. **Deterministic ordering helper (`docNamesInOrder`)**: Introduced `docNamesInOrder(cfg *Config) []string` in `internal/claude/init.go`. It returns doc names in a stable sequence:
   - First, preserving the explicit order specified in the top-level `docs:` list (`cfg.Docs`).
   - Second, any remaining configured language names in `cfg.AgentsMD.Languages` sorted lexicographically via `sort.Strings`.
2. **Auto-detection loop**: `autoDetectDocs` now iterates over `docNamesInOrder(cfg)` instead of directly iterating over `cfg.AgentsMD.Languages`.
3. **Doc name validation**: Added `validateDocNames(cfg *Config, names []string) error` ensuring unknown/misspelled doc names fail loudly with available options formatted via `docNamesInOrder`.

## Verification & Tests

- **Unit Test**: `TestDocNamesInOrder` in `internal/claude/docs_test.go` verifies:
  - Output matches expected order (`cfg.Docs` order followed by sorted remainder).
  - Iteration stability across multiple invocations (10 repeated loops asserting identical ordering).
- **Validation Test**: `TestValidateDocNames` in `internal/claude/docs_test.go` confirms unknown doc names produce helpful error messages and valid names pass.

