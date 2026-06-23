# Apply summary omits newly written commands

**Status:** Fixed (2026-06-23)

**Severity:** Low — misleading output, no data loss

## Problem

`ApplyAll` accumulated command names only in the `else` branch (unchanged path), so any
command that was written during the current run was excluded from the `commands:` summary
line.

```go
// before fix
if cr.changed {
    os.WriteFile(...)
    changes++
} else {
    presentCmds = append(presentCmds, cmd.Name)  // never reached on first apply
}
addStat("commands", strings.Join(presentCmds, ", "))
```

Result: after adding a new command and running `apply`, the summary showed the old commands
only — the newly installed one was invisible until the next (idempotent) run.

**Affected:** `internal/claude/apply.go` ~line 571

## Fix

Move the name accumulation outside the if/else so all configured commands are always listed:

```go
if cr.changed {
    os.WriteFile(...)
    changes++
}
cmdNames = append(cmdNames, cmd.Name)
addStat("commands", strings.Join(cmdNames, ", "))
```

Discovered when wiring `evergreen` command: the apply output confirmed the file was written
but the summary still showed only the three pre-existing commands.
