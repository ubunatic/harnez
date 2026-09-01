# 170 — Modularize `cmd/harnez/main.go` Command Definitions

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Code Quality / Refactor
**Related**: `cmd/harnez/main.go`, `cmd/harnez/find.go`, `cmd/harnez/exec.go`, `cmd/harnez/distill.go`, `cmd/harnez/rate.go`

---

## 1. Problem & Motivation

`harnez assess` flags `cmd/harnez/main.go` as a high-LOC (>500 LOC) file:
```
cmd/harnez/main.go: High LOC file (>500 LOC, consider splitting/modularizing)
cmd/harnez/main.go: High token weight (>4k tokens, agent context heavy)
```

While newer subcommands (`find`, `exec`, `distill`, `rate`, `repostatus`, `mode`, `stats`) live in their own dedicated files under `cmd/harnez/`, older commands (`usage`, `apply`, `diff`, `clean`, `assess`, `scan-docs`) and their respective helper logic remain embedded directly within `main.go`.

## 2. Technical Specification

Decompose `cmd/harnez/main.go` by extracting top-level command declarations and their local helpers into dedicated files in `package main`:

1. **`cmd/harnez/usage.go`**:
   - `usageCmd`, `agentCollectorCmd`
   - Usage flag parsing/validation helpers (`validateUsageFlags`, `resolveUsageHost`, etc.)
2. **`cmd/harnez/apply.go`**:
   - `applyCmd`, `diffCmd`, `cleanCmd`
3. **`cmd/harnez/assess.go`**:
   - `assessCmd`, `scanDocsCmd`
4. **`cmd/harnez/main.go`**:
   - Retain only `rootCmd`, `init()`, version flags, and core execution entry point `main()`.

### Constraints
- **Zero behavioral changes**: Flags, exit codes, help text, and default behaviors must remain byte-for-byte identical.
- Keep all new files in `package main` so internal command references and variables remain accessible without cyclical dependencies.

## 3. Verification Plan

1. Run `make test` across the project.
2. Run `harnez assess harnez` to verify that `cmd/harnez/main.go` no longer triggers the high-LOC warning.
3. Run `make install` and verify CLI subcommands (`harnez usage -h`, `harnez apply -h`, `harnez assess -h`).
