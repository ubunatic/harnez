# 1247 — Allow forge issues edit via CODEBERG_TOKEN

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 1202, 1118

---

/goal Allow `harnez issues edit` (and clarify/align forge write auth) to use `CODEBERG_TOKEN` (and configured `TokenEnv` env vars) when no stored write token is present, with tests and updated docs/tests, or stop and report when blocked on a user decision or denied permission. On a loop, hard blocker, or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

When running `harnez issues edit -d <repo> <n> -F body.md --if-updated <ts>` on a Codeberg forge-tracker repo without a separate stored write token (`harnez forge token create --write`), `harnez` fails with:

```
Error: no write token for codeberg.org; run: harnez forge token create --write
```

Even when `CODEBERG_TOKEN` is set in the environment (or `fj` is already authenticated), `issues edit` refuses to use `CODEBERG_TOKEN` because #1202 restricted forge writes to the stored write token only (`cmd/harnez/issues_forge.go:780,847`, `cmd/harnez/issues_forge_test.go:104`). Meanwhile, `issues new` and `issues comment` fall back to `fj`, forcing agents/users back to `fj -H codeberg.org issue edit <owner>/<repo>#<n> body ...` for edits.

## 2. Live Code Context

- `cmd/harnez/issues_forge.go:780,847`: returns `no write token for %s; run: harnez forge token create --write` when no stored write token is found.
- `cmd/harnez/issues_forge_test.go:104,698,976,1316`: tests asserting that `CODEBERG_TOKEN` is ignored for writes and that `no write token` is returned.
- `internal/forge/api.go` / `internal/forgetracker/write.go`: read path checks `TokenEnv` (`CODEBERG_TOKEN`, `FORGEJO_TOKEN`), while write path only checks the write token store.

## 3. Acceptance Criteria

- [ ] `harnez issues edit` (and forge write token resolution as appropriate) falls back to `CODEBERG_TOKEN` / configured `TokenEnv` when no stored write token is present (while still preferring the stored write token if configured, and preserving host-only session guards and secret-safe errors).
- [ ] If the env token lacks write scope (`401`/`403`), return a clear error pointing to `harnez forge token create --write` or `CODEBERG_TOKEN` scope without leaking token values.
- [ ] Update `cmd/harnez/issues_forge_test.go` and `internal/forgetracker/` tests to cover `CODEBERG_TOKEN` fallback for `edit`.
