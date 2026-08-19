# Permissions are grow-only — revoked entries never removed

**Status:** Open  
**Severity:** Medium — declarative config doesn't fully control permission state

## Problem

`mergePermissions` uses `unionStrings` for both allow and deny arrays, meaning a permission
removed from `config.yaml` stays in `settings.json` forever after the first apply.

The README's drift-repair section implies re-running apply "restores the managed keys",
which readers reasonably interpret as full reconciliation. In practice, permissions can only
accumulate — the only way to remove one is `clean` followed by `apply`.

**Affected:** `apply.go` ~line 181 (`mergePermissions` / `unionStrings`)

## Options

1. **Document it** — clarify in the README that permissions are union-merged and removal
   requires `clean` + `apply`. Lowest effort.

2. **Replace semantics for managed entries** — write the config's allow/deny sets directly,
   preserving only entries not present in the previous managed set (requires tracking which
   entries were written by claudeconfig vs added by the user/Claude Code).
