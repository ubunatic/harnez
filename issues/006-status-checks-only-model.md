# status checks only the model key — 6 of 7 managed settings invisible

**Severity:** Low — misleading output, not a data-loss bug

## Problem

The "Applied" section of `claudeconfig status` has one hardcoded probe for `settings.json`:

```go
checks := []entry{
    {
        label: "settings.json [model]",
        check: func() bool { return hasSettingsKey(settingsPath, "model") },
    },
}
```

hooks, env, permissions, effortLevel, mcpServers, and spinnerVerbs — all written by
`applyAll` — are never verified. Two failure modes:

- A config with only hooks and no model shows `settings.json [model]  missing` after a
  successful apply.
- Manually removing hooks from settings.json post-apply shows `ok` because model still
  exists.

**Affected:** `status.go` ~line 52

## Fix

Iterate `managedSettingsKeys` (already defined in `apply.go`) and add a `hasSettingsKey`
check for each key that is present in the built settings doc:

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
