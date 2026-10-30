# 1043 — Collector systemd unit lacks user PATH for agent CLIs

**Status**: Closed — Added user PATH to collector systemd unit and user-bin fallback resolver in usage package
**Closed-By**: codex:luna:med, session dotfiles-005-dev
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: `../dotfiles` issue 005, `systemd/harnez-agent-collector.service`, `internal/server/systemd.go`, `internal/usage/agy.go`

---

## 1. Problem & Motivation
`harnez-agent-collector.service` runs as a systemd user service (`harnez server`) to periodically collect agent usage and quota (including `agy`, `claude`, `codex`, `pi`).

In systemd user services, the service manager does not inherit the user's interactive shell `PATH`, and `environment.d` ignores `PATH` assignments. As a result, the collector unit runs with a minimal default PATH (`PATH=/usr/local/bin:/usr/bin`).

Most AI agent CLIs (`agy`, `claude`, `codex`, `pi`) and user-built binaries (`~/go/bin`, `~/.local/bin`, `~/.cargo/bin`) are installed in user directories (`%h/.local/bin`, `%h/go/bin`, `%h/.cargo/bin`). Because the shipped `harnez-agent-collector.service` sets no `Environment="PATH=..."`, the collector fails with:
`quota: unavailable (exec: "agy": executable file not found in $PATH)`
and marks AGY rows as `fetch fail`.

## 2. Technical Specification / Findings
1. **Systemd Unit Template**:
   - `systemd/harnez-agent-collector.service` defines the service.
   - Systemd user units expand the `%h` specifier to `$HOME` in `Environment=` values.
   - Setting `Environment="PATH=%h/.local/bin:%h/go/bin:%h/.cargo/bin:/usr/local/bin:/usr/bin:/bin"` provides standard user bin search paths to the collector.
   - Both `internal/claude/apply.go` and `internal/server/systemd.go` install this unit.
2. **Defensive Binary Resolution**:
   - `internal/usage/agy.go` runs `exec.CommandContext(ctx, "agy", "-p", "/usage")`.
   - If `agy` is not in the process `PATH`, resolving fallback paths (e.g., `~/.local/bin/agy`, `~/.gemini/antigravity-cli/bin/agy`) ensures robustness even if the collector is started outside systemd or under a degraded environment.
   - A helper function (e.g. `resolveAgentBin(ctx, "agy")` or looking in user bin dirs if `exec.LookPath` fails) makes binary resolution resilient across usage collectors.

## 3. Milestones

### M1: Configure PATH in systemd user service template
- In `systemd/harnez-agent-collector.service`, add:
  `Environment="PATH=%h/.local/bin:%h/go/bin:%h/.cargo/bin:/usr/local/bin:/usr/bin:/bin"`
- Verify that `internal/server/systemd.go` and `internal/claude/apply.go` install/substitute the unit cleanly without breaking existing placeholders (`{{HARNEZ_BIN}}`, etc.).
- Update any unit generation tests.

### M2: Fallback user-bin resolution in usage collector
- In `internal/usage/` (or a helper package), implement a fallback path resolver when searching for agent executables (specifically `agy`, but clean enough for reuse).
- If `exec.LookPath(name)` fails, check `~/.local/bin/<name>`, `~/go/bin/<name>`, and `~/.cargo/bin/<name>`.
- Add unit tests covering:
  - Resolving binary in PATH.
  - Resolving binary in `~/.local/bin` when PATH is stripped/minimal.
- Update `internal/usage/agy.go` to use this resolver.

### M3: Verification & Integration
- Run `make test-short` and `go test ./internal/server/... ./internal/usage/...`.
- Verify `systemd/harnez-agent-collector.service` syntax.
- Verify `harnez usage --agent agy` resolution behavior.
