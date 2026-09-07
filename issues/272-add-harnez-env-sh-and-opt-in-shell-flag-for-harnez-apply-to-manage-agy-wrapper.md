# 272 — Add ~/.harnez/env.sh and opt-in --shell flag for harnez apply to manage agy wrapper

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture / CLI
**Related**: Issue 270; Issue 271; `internal/claude/apply.go`; `cmd/harnez/main.go`

---

## 1. Problem & Motivation

With Issue 271, `harnez apply` provisions the guarded `bash` shim at `~/.harnez/shims/bash` for quiet tool telemetry in Antigravity (`agy`). To activate this shim cleanly without polluting terminal tab/window titles with inline environment variables (`ANTIGRAVITY_AGENT=1 agy`), users need a shell function wrapper:

```bash
agy() {
    PATH="$HOME/.harnez/shims:$PATH" ANTIGRAVITY_AGENT=1 command agy "$@"
}
```

Manually creating and keeping this function synchronized across user dotfiles (`~/.bashrc`, `~/.zshrc`) is error-prone. Providing an official `~/.harnez/env.sh` source script managed by `harnez apply`, alongside an opt-in `harnez apply --shell` flag to wire the source line into shell rc files, provides a clean, self-contained, and automated setup.

## 2. Scope

**In scope:**
- `~/.harnez/env.sh`: Generated and kept up-to-date on `harnez apply`. Contains managed `agy()` shell function wrapper with guarded shims PATH and agent flag.
- `harnez apply --shell` (`-s`): Opt-in flag that injects/updates `# harnez:begin env` ... `# harnez:end env` in `~/.bashrc` (and `~/.zshrc` if present).
- `harnez diff` / `harnez status`: Check and report `~/.harnez/env.sh` and shell rc integration status and drift.
- `harnez clean`: Clean managed `env` blocks from `~/.bashrc` / `~/.zshrc` and remove `~/.harnez/env.sh`.
- Unit and integration tests covering env generation, drift detection, and shell rc injection.

**Out of scope:**
- Modifying dotfiles by default when `--shell` flag is not passed.

## 3. Acceptance Criteria

- [x] `harnez apply` creates/updates `~/.harnez/env.sh` containing the `agy()` wrapper function bounded by `# harnez:begin env`.
- [x] `harnez apply --shell` (`-s`) idempotently injects the source snippet into `~/.bashrc` (and `~/.zshrc` if present).
- [x] Plain `harnez apply` (without `--shell`) does NOT modify `~/.bashrc` or `~/.zshrc`.
- [x] `harnez status` accurately reports status of `~/.harnez/env.sh` and shell rc integration.
- [x] `harnez clean` removes `~/.harnez/env.sh` and strips managed sections from shell rc files.
- [x] All test suites (`make check`, `scripts/smoke-test.sh`) pass 100%.

