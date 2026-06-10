# mergeLangs dedup bug — duplicate --lang flags pass through

**Severity:** Medium — duplicate langs passed to applyAll

## Problem

`mergeLangs` checks `seen` in the second loop but never updates it, so duplicate `--lang`
flags are not deduplicated:

```go
for _, l := range fromFlag {
    if _, ok := seen[l]; !ok {
        result = append(result, l)  // appends but does NOT set seen[l]
    }
}
```

`claudeconfig apply --lang go --lang go` produces `["go", "go"]` passed to `applyAll`,
causing the language file to be copied and symlinked twice.

Compare: `unionStrings` in `apply.go` correctly sets `seen[s] = struct{}{}` in both loops.

**Affected:** `main.go` lines 20–24

## Fix

Either add `seen[l] = struct{}{}` after the append, or replace the body of `mergeLangs`
with a call to `unionStrings` (which already exists in `apply.go` and handles this correctly).
