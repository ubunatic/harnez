## Summary
1 finding: `trackerDirty` fails to detect when a tracked ticket is renamed to an untracked directory, allowing reconcile/rebase to run on a dirty worktree.

## Findings
### `trackerDirty` ignores rename sources, missing tracked dirty state
- `internal/issues/maintenance/rebase.go:131`
- `if path == ".gitattributes" || path == "issues" || strings.HasPrefix(path, "issues/") {`
- Trigger: `git mv issues/001-a.md docs/001-a.md`. The porcelain status is `R  issues/001-a.md -> docs/001-a.md`. The arrow logic in `trackerDirty` (`if arrow := strings.LastIndex(path, " -> "); arrow >= 0 { path = path[arrow+4:] }`) extracts the destination (`docs/001-a.md`), which does not start with `issues/`.
- `trackerDirty` returns false, meaning reconcile and rebase will proceed despite tracked changes to issues. It should detect that the source file is an issue and return true.
- Confidence: confirmed by running it

## Not checked
Most of the merge logic and Git index manipulation.
