# Case Study: Usage History Subcommands, Process Telemetry Panel, and Remote Host Monitoring

**Date**: 2026-08-23  
**Scope**: `cmd/harnez/main.go`, `internal/usage/`, `Makefile`, issue tickets 048–051  
**Author**: Antigravity (Pair Programming with User)

---

## 1. Executive Summary

This session undertook three core functional expansions and architectural cleanups in `harnez`:
1. **Usage History Command Decomposition (048)**: Extracted bloated, conflicting CLI flags (`--history`, `--timeline`, `--fetch`) out of `harnez usage` into clean, dedicated subcommands under `harnez usage history [timeline|fetch|record|stats]`.
2. **Agent Process Telemetry Status Panel (049)**: Added local process discovery (inspecting `/proc` on Linux with fallback) to count active `claude`, `agy`, and `codex` processes. Rendered in a new `[P] Processes` panel in the TUI, hidden by default and activated via `--proc`/`-p` or hotkey `[p]`/`[P]`/`6` in live `--watch` mode.
3. **Remote Host Usage & Process Querying (050)**: Added `--host <ssh-target>` flag and `[r]` / `[R]` hotkey to `--watch` mode to query and switch to a remote machine's metrics via SSH streaming JSON (`harnez usage --json [--proc]`).
4. **Remote Host Synchronization (`make sync`)**: Added a convenient `make sync` target to the project Makefile to streamline local commits, remote git pulls, binary rebuilds, and verification across test rigs (e.g. `um760`).

All 44 tracked issues in `issues/README.md` remain strictly in sync, and all unit test suites pass (`go test ./...`).

---

## 2. Architectural Decisions & What Worked Well

### 2.1 Cobra Subcommand Hierarchy vs. Flag Explosion
Placing timeline rendering, remote log fetching, and snapshot recording behind boolean and string flags on `harnez usage` produced a combinatorial explosion of validation rules (`--timeline` vs `--watch` vs `--summary`). 

Refactoring these to:
- `harnez usage history` (default / timeline)
- `harnez usage history fetch <host>`
- `harnez usage history record`
- `harnez usage history stats`

eliminated the flag conflict matrix, simplified CLI help docs, and established a scalable foundation for analytics (e.g., retention, log pruning, sparkline stats).

### 2.2 Low-Overhead Local Process Discovery
Rather than introducing heavy process inspection dependencies or parsing volatile multi-column `ps` outputs, `CountRunningAgentProcesses()` directly inspects `/proc/[pid]/comm` and `/proc/[pid]/cmdline` on Linux. It isolates target executables (`claude`, `agy`, `codex`) and only falls back to `ps -eo comm=` on non-Linux platforms or restricted environments.

### 2.3 Non-Intrusive SSH JSON Bridge
Remote monitoring executes `ssh -q -o BatchMode=yes -o ConnectTimeout=5 <host> PATH="..." harnez usage --json [--proc]`.
- Reuses standard SSH configuration (`~/.ssh/config`) without needing custom ports, keys, or daemons.
- Emits structured JSON directly into our internal decoders, reusing existing TUI renderers without duplicating formatting code.
- If SSH drops, times out, or fails authentication, the UI catches the error and surfaces a standard `QuotaFetchError` badge without crashing or hanging the TUI event loop.

---

## 3. Honest Post-Mortem & Near-Misses

### 3.1 Remote Non-Login Shell `PATH` Ingestion Gap
- **Hazard**: When running `make sync` (`ssh um760 "cd projects/harnez && git pull && make install && harnez status"`), the remote SSH session ran in non-login / non-interactive mode. In this mode, `$HOME/go/bin` is not automatically in `PATH`, causing `zsh:1: command not found: harnez`.
- **Catch**: Immediate failure during `make sync` execution.
- **Fix**: 
  1. Updated `Makefile` `sync` recipe to invoke `make status`, which uses the locally built relative binary `./harnez status` rather than assuming `$HOME/go/bin` in global path.
  2. Verified that `internal/usage/remote.go` already explicitly prepends `PATH="$PATH:$HOME/go/bin:$HOME/bin:/usr/local/bin"` when launching remote `harnez` commands.

### 3.2 TDD & Parallel Subagent Execution
- Utilizing parallel / sequential subagent delegation with explicit task boundaries (`048`, `049`, `050`) allowed rapid feature implementation, robust test coverage creation, and clean ticket synchronization without cluttering the main orchestrator's context window.

---

## 4. Quality & Invariants Audit

| Check | Result | Details |
|---|---|---|
| **Test Suite** | Pass | `go test ./...` passes cleanly across all packages in ~4.1s. |
| **Idempotency** | Pass | `harnez apply`, `harnez diff`, and `harnez status` report clean state with 0 spurious drift. |
| **Issue Tracker** | Pass | All 44 tickets reconciled and verified via `harnez status`. |
| **CLI Ergonomics** | Pass | Flags cleanly separated; help output tested; TUI keybindings verified. |

---

## 5. File Summary

- **`cmd/harnez/main.go`**: Removed deprecated history flags; added `history` subcommands suite, `--proc` / `-p` flags, and `--host` flag.
- **`internal/usage/history.go` & `history_test.go`**: Added `RenderHistoryStatsText` and `RenderHistoryStatsJSON` helpers.
- **`internal/usage/process.go` & `process_test.go`**: Implemented process inspection via `/proc` and `ps`.
- **`internal/usage/remote.go` & `remote_test.go`**: Implemented SSH remote JSON query and payload decoding.
- **`internal/usage/watch.go` & `watch_test.go`**: Added `[P] Processes` box, `[r]` / `[R]` remote host switcher hotkey, and remote target header rendering.
- **`Makefile`**: Added overridable `sync` target (`HOST`, `HOST_DIR`).
- **`issues/`**: Created & resolved tickets 048, 049, 050; created open ticket 051.
