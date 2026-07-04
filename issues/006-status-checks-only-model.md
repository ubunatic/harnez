# status settings.json check covers only model key

**Status:** Open — partially fixed

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
