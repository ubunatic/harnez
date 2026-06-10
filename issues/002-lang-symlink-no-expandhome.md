# Language symlink path not expandHome'd — literal ~ directory created

**Severity:** High — silently writes to wrong location

## Problem

Global symlinks correctly call `expandHome(g.Symlink)` before passing to `ensureSymlink`,
but language symlinks pass `lang.Symlink` raw:

```go
// global — correct:
link := expandHome(g.Symlink)
lr, err := ensureSymlink(link, gTarget)

// language — bug:
slr, err := ensureSymlink(lang.Symlink, dst)
```

If `lang.Symlink` is `"~/projects/foo/docs/Go.md"`, `ensureSymlink` calls
`os.MkdirAll("~", 0755)` which creates a literal `~` directory in the working directory.

**Affected:** `apply.go` ~line 762

## Fix

Wrap the lang symlink path: `ensureSymlink(expandHome(lang.Symlink), dst)`
