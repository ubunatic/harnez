# mergeDocs dedup bug — duplicate --docs flags pass through

**Severity:** Medium — duplicate doc names passed to ApplyAll/RunInit

## Problem

`mergeDocs` checks `seen` in the second loop but never updates it, so duplicate `--docs`
flags are not deduplicated:

```go
for _, l := range fromFlag {
    if _, ok := seen[l]; !ok {
        result = append(result, l)  // appends but does NOT set seen[l]
    }
}
```

`claudeconfig apply --docs golang --docs golang` produces `["golang", "golang"]` passed to `ApplyAll`,
causing the doc file to be written twice.

Compare: `unionStrings` in `apply.go` correctly sets `seen[s] = struct{}{}` in both loops.

**Affected:** `apply.go` (`mergeDocs` function)

## Fix

Either add `seen[l] = struct{}{}` after the append, or replace the body of `mergeDocs`
with a call to `unionStrings` (which already exists in `apply.go` and handles this correctly).

## Note

This function was renamed from `mergeLangs` to `mergeDocs` on 2026-07-04 as part of the
`--lang` → `--docs` flag rename. The bug was present in the original and carried over.
