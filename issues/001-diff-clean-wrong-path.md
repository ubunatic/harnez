# diff and clean operate on wrong file when --project is set

**Status:** Resolved — `-p`/`--project` removed from `apply`/`diff`/`clean` (2026-06-23); project work moved to `init`

**Severity:** High — silent wrong-file operations (was)

## Problem

`applyAll` resolves the local AGENTS.md path via `localPath(projectDir, l.Target)`, but
`diffAll` and `cleanAll` pass `l.Target` raw. Neither function accepts a `projectDir`
parameter.

Running `claudeconfig diff` or `claudeconfig clean` after an `apply --project ./myproj`
reads/modifies the CWD's AGENTS.md instead of the project's — no error, wrong file.

**Affected:** `apply.go` ~line 819 (`diffAll`), ~line 845 (`cleanAll`)

## Fix

Add `projectDir string` to both function signatures and apply `localPath(projectDir, l.Target)`
identically to `applyAll` (~line 681).

Also affects `status.go` ~line 67: `runStatus` uses `cfg.AgentsMD.Local.Target` raw for the
local-section applied-state check, same root cause.
