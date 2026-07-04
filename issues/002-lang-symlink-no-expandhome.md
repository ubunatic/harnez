# Language symlink path not expandHome'd — literal ~ directory created

**Status:** Closed — invalid (2026-07-04)

**Severity:** N/A

## Why closed

The `Language` struct has no `Symlink` field. The `lang.Symlink` code path described here
does not exist in the current codebase — `Language` only has `Name`, `Ref`, `Hint`,
`Source`, `Target`, `Local`, `Template`, and `Targets`.

The `Symlink` field exists only on `AgentsMDTarget` (global/local), and the global case
already calls `fsutil.ExpandHome(g.Symlink)` correctly. The local symlink (`CLAUDE.md`) is
a relative path that does not require home expansion.

No action needed.
