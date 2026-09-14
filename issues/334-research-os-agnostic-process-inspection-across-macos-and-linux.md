# 334 — Research OS-agnostic process inspection across macOS and Linux

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Functional Gap (Platform Portability)
**Category**: Architecture / Multi-OS
**Related**: [docs/MacOSPortability.md](../docs/MacOSPortability.md)

---

## 1. Problem & Motivation

`harnez` currently counts active AI coding agent processes (`claude`, `agy`, `codex`, `pi`, `opencode`) using `exec.Command("ps", "-eo", "comm=")` in `internal/usage/process.go`.

While `ps -eo comm=` works on standard GNU/Linux, relying on fork/exec of the `ps` CLI tool introduces several limitations:
1. **Subprocess Overhead**: Shelling out to `ps` on every polling interval is inefficient compared to direct kernel/process table inspection.
2. **Flag & Output Incompatibilities**: BSD/macOS `ps` differs in formatting and flags compared to GNU `ps` (e.g. `comm` vs `command`, headers handling, truncation of long command names).
3. **Container & Sandbox Restrictions**: Minimal environments may omit `ps` or restrict process table visibility.

We need to research how top-tier monitoring and system inspection tools implement robust, cross-platform process discovery across Linux and macOS.

## 2. Research Scope & Prior Art

Investigate how the following tools and libraries handle process listing on Linux and macOS (Darwin):
- **`htop`**: Linux `/proc` parsing vs Darwin `sysctl(KERN_PROC, KERN_PROC_ALL)` and `proc_listpids()` / `proc_pidinfo()`.
- **`btop`**: Modern C++ multi-platform process inspection.
- **Go ecosystem standards**:
  - `golang.org/x/sys/unix` direct `sysctl` bindings for Darwin.
  - `gopsutil` / `shirou/gopsutil` process table implementation strategies.
  - Direct Linux `/proc/[pid]/comm` and `/proc/[pid]/stat` scanning vs BSD `sysctl`.

## 3. Key Questions to Answer

1. Can we perform process discovery purely in Go without shelling out to `ps` on either Linux or macOS?
2. What are the performance and permission differences between `procfs` on Linux vs `sysctl` / `libproc` on macOS?
3. What is the recommended Go abstraction (e.g., build-tag split `process_linux.go` and `process_darwin.go` vs a shared lightweight abstraction)?

## 4. Deliverables

- Write a research study in `docs/studies/` summarizing findings, benchmarks, and recommended architecture.
- File follow-up implementation ticket(s) to replace `ps -eo comm=` with the chosen cross-platform mechanism.
