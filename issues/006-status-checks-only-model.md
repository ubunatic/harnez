# status settings.json check covers only model key

**Status:** Closed — resolved in a39b916

**Severity:** Low — misleading output, not a data-loss bug

## What was fixed

`status` now checks sections (global + local), commands, and skills — those are all
iterated dynamically from config.

## Remaining gap

The `settings.json` block still has one hardcoded probe:

```go
checks := []entry{
    {
        label: "settings.json [model]",
        check: func() bool { return hasSettingsKey(settingsPath, "model") },
    },
}
```

`hooks`, `env`, `permissions`, `effortLevel`, `mcpServers`, and `spinnerVerbs` — all written
by `applyAll` — are never verified. Two failure modes remain:

- A config with only hooks and no model shows `settings.json [model]  missing` after a
  successful apply.
- Manually removing hooks from settings.json post-apply still shows `ok` because model
  exists.

**Affected:** `status.go` ~line 48

## Fix

Iterate `managedSettingsKeys` (already defined in `apply.go`) and add a `hasSettingsKey`
check for each key present in the built settings doc:

```go
for _, k := range managedSettingsKeys {
    k := k
    if _, inDoc := settingsDoc[k]; inDoc {
        checks = append(checks, entry{
            label: "settings.json [" + k + "]",
            check: func() bool { return hasSettingsKey(settingsPath, k) },
        })
    }
}
```

Replace the hardcoded `model` entry with this loop (model is covered by `managedSettingsKeys`).

---

## Implementation Plan

The gap is still present: `internal/claude/status.go:64-68` hardcodes the single
`settings.json [model]` probe, while `managedSettingsKeys` (`internal/claude/apply.go:96`)
already lists `model, effortLevel, permissions, hooks, env, spinnerVerbs, mcpServers,
statusLine`. The ticket's proposed fix is correct as written; this plan just pins the details.

### Steps

1. **`internal/claude/status.go`** — in `RunStatus`, before building `checks`, build the doc
   once: `doc := buildSettingsDoc(cfg)`. Replace the hardcoded entry with:
   ```go
   var checks []entry
   for _, k := range managedSettingsKeys {
       k := k
       if _, inDoc := doc[k]; !inDoc {
           continue // config doesn't declare it — nothing to verify
       }
       checks = append(checks, entry{
           label: "settings.json [" + k + "]",
           check: func() bool { return hasSettingsKey(settingsPath, k) },
       })
   }
   ```
   Gating on `inDoc` is what fixes failure mode 1 (a config with no model no longer reports
   `missing`); iterating all keys fixes failure mode 2 (removed hooks now report `missing`).

2. **Avoid re-reading settings.json per key** — `hasSettingsKey` calls `jsonc.Read` on every
   invocation, so the loop would parse the file up to 8 times. Read it once above the loop
   (`applied := jsonc.Read(settingsPath)`) and close over `applied` with a
   `_, ok := applied[k]` check. Keep `hasSettingsKey` for other callers, or drop it if this
   was its only use (grep shows `status.go` is the only non-test caller).

3. **Presence vs. equality** — keep the check at *presence*, matching current semantics.
   Deep-equality against the built doc is what `harnez diff` is for; duplicating it in
   `status` would make `status` and `diff` two half-overlapping drift detectors.

4. **Tests**
   - `internal/claude/telemetry_hook_test.go:135` asserts on the literal string
     `"settings.json [model]"` — still valid, since `model` remains in the doc there.
   - `internal/claude/integration_test.go:212` likewise; leave both.
   - Add `TestRunStatus_ChecksAllManagedSettingsKeys` in `internal/claude/status_test.go`
     (new file): apply a config that declares hooks + permissions but no model, capture
     `RunStatus` output, assert it contains `settings.json [hooks]  ok` /
     `settings.json [permissions]` and does **not** contain `settings.json [model]`.
   - Add `TestRunStatus_ReportsRemovedManagedKey`: after apply, delete `hooks` from
     `settings.json`, assert `status` reports `settings.json [hooks]` as missing.

### Design decisions

- **Data-driven off `managedSettingsKeys`** — single source of truth shared with `applyAll`,
  `cleanSettingsJSON`, and the diff notes builder, so a future managed key is covered by all
  four automatically.
- **Skip keys absent from the built doc** rather than reporting them missing: `status`
  answers "is what my config declares actually applied?", not "does settings.json have
  everything harnez could write?".

### Risks / open questions

- Output grows from one line to up to eight; check it still reads well in the `Applied:`
  block alongside the per-command/per-skill lines. If noisy, consider collapsing to one
  `settings.json [model, hooks, env]` line — but only if the user complains, not preemptively.
- `statusLine` is written conditionally; confirm `buildSettingsDoc` omits the key entirely
  (rather than emitting an empty value) when unconfigured, else it will always show.

### Scope

**Small** — ~20 LOC in `status.go` plus two focused tests.
