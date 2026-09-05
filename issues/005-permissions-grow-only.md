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

---

## Implementation Plan

Chosen option: **2 (replace semantics for managed entries)**, with option 1's doc fix folded in.
Union-merge must stay for entries harnez never wrote — Claude Code itself appends to
`permissions.allow` whenever the user approves a prompt, and `apply` must not delete those.
Full reconciliation therefore requires remembering what harnez wrote last time.

### Steps

1. **State sidecar** — new small file `internal/claude/permstate.go`:
   - `permStatePath(target) = filepath.Join(target, ".harnez", "managed-permissions.json")`
   - `readPermState(target) (allow, deny []string)` — missing/corrupt file returns empty
     (degrade gracefully, never hard-error; matches the resilient-resolution norm).
   - `writePermState(target, allow, deny []string) error` — `0644`, created after a
     successful `applySettingsJSON`.

2. **Reconcile in `mergePermissions`** (`internal/claude/apply.go` ~62) — change signature to
   `mergePermissions(existing, incoming map[string]any, prevManaged permState) map[string]any`.
   Per field (`allow`, `deny`):
   ```
   kept  := existing[field] minus prevManaged[field]   // user/Claude-Code additions survive
   final := union(kept, incoming[field])               // config wins, order-stable
   ```
   With an empty `prevManaged` this is byte-identical to today's union — so the first apply
   after upgrade is a no-op, and reconciliation starts working from the second apply on.

3. **Thread it through** — `applyMerge` (~74) takes the same `prevManaged` argument;
   `applySettingsJSON` (~318) reads the state before merging and writes it after a successful
   write, storing the *config's* allow/deny (i.e. `incoming`), not the merged result.

4. **Diff parity** — `diffSettingsJSON` / the notes builder (~185) must use the same
   reconciled merge, otherwise `diff` would under-report removals. Extend the existing
   `permissions.<field>: %+d entries` note to also name removed entries, e.g.
   `permissions.allow: -1 entries (12 → 11), removed: Bash(rm:*)`.

5. **`cleanSettingsJSON`** (~296) — also remove `<target>/.harnez/managed-permissions.json`
   (and the `.harnez` dir if empty), so `clean` + `apply` still behaves as a full reset.

6. **Tests** — `internal/claude/apply_test.go` (new) table test on the merge helper:
   - no prior state → pure union (back-compat)
   - entry dropped from config, present in prior state → removed
   - entry absent from config, absent from prior state (user-added) → preserved
   - entry in both config and prior state, still in config → kept once, no duplication
   Plus one integration-level case in `internal/claude/integration_test.go`: apply, remove a
   permission from config, apply again, assert it is gone and a user-added one survives.

7. **Docs** — README drift-repair section: state that permissions are now reconciled against
   the last managed set, that user/Claude-Code-approved entries are never removed, and that
   the first apply after upgrading is still a union.
   `docs/Permissions.md` gets the same one-paragraph note.

8. **Smoke** — `scripts/smoke-test.sh` already has `scripts/drop-perm.sh` for drift
   simulation; add the mirror case (drop an entry from a temp `config.yaml`, apply, assert it
   disappears from `settings.json`).

### Design decisions / tradeoffs

- **Sidecar over an in-settings key**: `settings.json` is read by Claude Code, which may warn
  on or strip unknown top-level keys. A separate file under `<target>/.harnez/` keeps harnez
  bookkeeping out of a foreign schema, and is trivially removable by `clean`.
- **Union stays the fallback**: no state file ⇒ old behaviour. This makes the change
  non-breaking and means a deleted state file degrades to "grow-only" rather than to
  "delete the user's approvals".
- **Config is authoritative only over what it previously owned** — deliberately *not* a full
  replace, which would silently wipe every permission the user approved interactively.

### Risks / open questions

- A permission that appears in *both* the config and the user's manual additions becomes
  indistinguishable; removing it from config will remove it from settings. Acceptable — the
  user can re-approve.
- `.harnez/` inside `~/.claude` is a new directory; confirm nothing else scans that tree and
  chokes on it (nothing does today).
- Should `deny` reconcile identically? Yes for symmetry, but removing a deny entry is
  security-relevant — worth an explicit line in the diff output so it is never silent.

### Scope

**Medium** — ~150 LOC across `apply.go` + a new `permstate.go`, plus tests, README and
`docs/Permissions.md` updates.
