# 696 — harnez read: support direct ticket number targets and error hints for malformed -L line ranges

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Ergonomics / Tooling
**Related**: 695 (positional resolution), 626 (harnez read auto), 638 (harnez read missing rules)

---

## 1. Problem & Motivation
In subagent trajectories on 2026-10-03, developer subagents repeatedly encountered tool call failures when invoking `harnez read`:
1. **Malformed `-L` range argument**: Workers wrote `harnez read -L 695` or `harnez read -L internal/cli/classify.go`, causing Cobra to consume the target path as the `-L` range value. When `file.go` failed range parsing or left the file arguments empty, the command either hung reading `stdin` or emitted a non-obvious syntax error.
2. **Ticket Path Verbosity**: When inspecting tickets, agents must resolve `issues/<NNN>-<slug>.md` manually (or via `ls issues/`) before running `harnez read`. Supporting bare numeric ticket targets (e.g. `harnez read 696` or `harnez read -L 1:40 696`) avoids extra `ls`/`find` tool turns.

## 2. Technical Specification & Ergonomics

### A. Direct Ticket Number Resolution
- If a positional argument to `harnez read` is an integer or 3-digit zero-padded ticket number (e.g. `696` or `041`):
  - Check if `issues/<NNN>-*.md` exists in the current repo.
  - If exactly one matching ticket file is found, resolve the argument to that path.

### B. Flag Error & Range Parsing Hints
- If `-L <val>` receives a value containing `/` or `.go`/`.md` (a file path rather than `<start>:<end>` or `<start>-<end>`):
  - Emit a clear, immediate error: `harnez read: invalid line range "<val>"; -L expects <start>:<end> (e.g. -L 1:50 <file>)`.
- Prevent hanging on `stdin` when `-L` is provided without positional file arguments and `stdin` is a terminal.

## 3. Implementation & Verification Plan
- Update argument and file resolution in `cmd/harnez/read.go`.
- Add range validation helper with path-like pattern detection in `cmd/harnez/read.go`.
- Unit tests in `cmd/harnez/read_test.go`.
