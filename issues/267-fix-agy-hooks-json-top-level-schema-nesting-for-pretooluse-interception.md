# 267 — Fix agy hooks.json Top-Level Schema Nesting for PreToolUse Interception

**Status**: Open — filed via /issue
**Priority**: P2 (Medium)
**Severity**: Normal (fixes missing PreToolUse command interception in agy)
**Category**: Hooks / Multi-Harness Integration
**Related**: [[196-agy-native-hooks-plan-alongside-claude-hooks]], [[266-support-antigravity-session-and-agent-id-resolution-in-telemetry]], `internal/agy/hooks.go`, `cmd/harnez/agyhooks.go`

---

## 1. Problem & Motivation

In [[196]], `harnez agy-hooks apply` was created to install a `PreToolUse` hook for `run_command` in `~/.gemini/config/hooks.json`.

However, `internal/agy.BuildHooksDoc` generated the hook schema wrapped under an extra `"hooks"` key:
```json
{
  "hooks": {
    "harnez": {
      "enabled": true,
      "PreToolUse": [ ... ]
    }
  }
}
```

Per Antigravity's documented hook format (`agy-customizations/docs/hooks.md`), `hooks.json` requires hook names to be **top-level keys**:
```json
{
  "harnez": {
    "enabled": true,
    "PreToolUse": [
      {
        "matcher": "run_command",
        "hooks": [
          { "type": "command", "command": "harnez agy-hooks hook" }
        ]
      }
    ]
  }
}
```

Because of the invalid `"hooks"` root wrapper, `agy` never matched the `harnez` hook, so `run_command` steps ran raw without being rewritten into `harnez exec`.

---

## 2. Technical Design & Architecture

### 2.1 Top-Level Hook Schema Generation & Merge
In `internal/agy/hooks.go`:
1. `BuildHooksDoc()`:
   Return `map[string]any{HookName: map[string]any{...}}` directly at top-level.
2. `mergeHooksDoc(existing, incoming)`:
   - Merge `HookName` at the root dictionary.
   - Clean up any legacy nested `"hooks": { "harnez": ... }` if present in existing files.
   - Preserve any other user-defined named hooks (e.g. `lint-checker`).
3. `extractHook(doc)` & `Status(path)`:
   - Inspect `doc[HookName]` at root.
   - Flag legacy `"hooks"` wrapper as drifted/requiring repair.

---

## 3. Scope of Implementation

1. **`internal/agy/hooks.go`**:
   - Fix schema structure in `BuildHooksDoc`, `mergeHooksDoc`, `extractHook`, and `Delete`.
2. **`internal/agy/hooks_test.go`**:
   - Update tests to verify top-level hook structure, migration from legacy wrapper, and status/drift checks.
3. **Run `harnez agy-hooks apply`**:
   - Update live `~/.gemini/config/hooks.json` to the correct schema.

---

## 4. Acceptance Criteria

- [ ] `~/.gemini/config/hooks.json` is generated with `"harnez"` as a top-level key.
- [ ] `harnez agy-hooks status` reports `up to date`.
- [ ] `internal/agy` unit tests pass cleanly.
- [ ] `make check` and `make install` pass.

---

## 5. Verification

- **Automated**: `go test -v ./internal/agy/...`
- **Manual Verification**: Run `harnez agy-hooks apply && cat ~/.gemini/config/hooks.json` and verify top-level JSON structure.

