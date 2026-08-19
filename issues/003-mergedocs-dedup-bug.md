# mergeDocs dedup bug — duplicate --docs flags pass through

**Status:** Closed — fixed in `internal/claude/apply.go` and `internal/jsonc/jsonc.go` (2026-08-19)
**Severity:** Medium — duplicate doc names pass through mergeDocs to ApplyAll

## Problem

`mergeDocs` in [`internal/claude/apply.go`](../internal/claude/apply.go) checks `seen` in the second loop (`fromFlag`) but never adds new elements to `seen`. Consequently, duplicate `--docs` / `-d` CLI flags (e.g. `--docs golang --docs golang` or `--docs golang,golang`) are not deduplicated if they were not already present in `fromConfig`:

```go
func mergeDocs(fromConfig, fromFlag []string) []string {
	seen := make(map[string]struct{}, len(fromConfig)+len(fromFlag))
	result := make([]string, 0, len(fromConfig)+len(fromFlag))
	for _, l := range fromConfig {
		seen[l] = struct{}{}
		result = append(result, l)
	}
	for _, l := range fromFlag {
		if _, ok := seen[l]; !ok {
			result = append(result, l) // appends but does NOT record seen[l] = struct{}{}
		}
	}
	return result
}
```

### Impact & Callers

- `mergeDocs` is called inside `ApplyAll` in [`internal/claude/apply.go`](../internal/claude/apply.go):
  ```go
  globalDocs := mergeDocs(cfg.Docs, docs)
  ```
- Running `harnez apply --docs golang --docs golang` produces `globalDocs = ["golang", "golang"]`, causing the doc file to be installed / processed twice.
- Note on `RunInit`: `RunInit` in [`internal/claude/init.go`](../internal/claude/init.go) receives CLI docs directly from Cobra without calling `mergeDocs` (and appends `autoDetectDocs`), so `RunInit` does not use `mergeDocs`, but passing duplicates to `RunInit` or `ApplyAll` is also not guarded at the CLI boundary.

Compare: [`jsonc.UnionStrings`](../internal/jsonc/jsonc.go) correctly marks `seen[s] = struct{}{}` in both loops.

**Affected:** [`internal/claude/apply.go`](../internal/claude/apply.go) (`mergeDocs` function)

## Recommended Fix

Replace the body of `mergeDocs` with a direct call to `jsonc.UnionStrings` (or delegate to it), since `jsonc.UnionStrings` already implements identical semantics and deduplication correctly:

```go
func mergeDocs(fromConfig, fromFlag []string) []string {
	return jsonc.UnionStrings(fromConfig, fromFlag)
}
```

Alternatively, fix the loop directly by setting `seen[l] = struct{}{}` inside the `fromFlag` branch:

```go
	for _, l := range fromFlag {
		if _, ok := seen[l]; !ok {
			seen[l] = struct{}{}
			result = append(result, l)
		}
	}
```

Using `jsonc.UnionStrings` is preferred to eliminate duplicate logic.

## Test Plan

Currently, there are no unit tests covering `mergeDocs` in `internal/claude/docs_test.go` or `apply_test.go`.

Add a unit test `TestMergeDocs` to [`internal/claude/docs_test.go`](../internal/claude/docs_test.go):
1. **Duplicate flag entries:** `mergeDocs([]string{"golang"}, []string{"make", "make"})` → `[]string{"golang", "make"}`
2. **Flag duplicates config:** `mergeDocs([]string{"golang", "make"}, []string{"golang", "canary"})` → `[]string{"golang", "make", "canary"}`
3. **Multiple duplicates in flag:** `mergeDocs(nil, []string{"canary", "canary", "canary"})` → `[]string{"canary"}`
4. **Empty inputs:** `mergeDocs(nil, nil)` → `[]string{}` (or empty slice)

## Resolution

1. Replaced `mergeDocs` in `internal/claude/apply.go` with `jsonc.UnionStrings(fromConfig, fromFlag)` to deduplicate doc names and reuse common logic.
2. Updated `jsonc.UnionStrings` in `internal/jsonc/jsonc.go` to deduplicate items within both input slices while preserving first-seen order.
3. Added unit tests for `mergeDocs` in `internal/claude/docs_test.go` and for `UnionStrings` / `ToStrings` in `internal/jsonc/jsonc_test.go`.

## Note

This function was renamed from `mergeLangs` to `mergeDocs` as part of the `--lang` → `--docs` flag refactor. The bug was present in the original implementation and carried over.

