# 438 — harnez agent enable and disable commands to toggle subagent dispatch policy in AGENTS.local.md and coordinate with init

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agent Orchestration / Configuration Management
**Related**: #437, #311, #342

---

## 1. Problem & Motivation

Currently, setting or toggling the subagent dispatch policy (e.g. `subagent_mode: harnez` vs `subagent_mode: native`) in a repository requires hand-editing `./AGENTS.local.md` (or `./AGENTS.md`). There is no CLI command under `harnez agent` to declaratively enable or disable the `harnez agent` dispatch policy.

Furthermore:
1. `harnez init` and `harnez agent [enable|disable]` must be strictly coordinated:
   - `harnez init` ensures `<!-- harnez:begin Local Overlays -->` is present in `AGENTS.md` and `AGENTS.local.md` is excluded via `.git/info/exclude`.
   - `harnez agent enable` / `harnez agent disable` should manage a well-defined `<!-- harnez:begin Subagent Policy -->` block in `AGENTS.local.md` (or `AGENTS.md` with `--persist`).
   - Running `harnez init` must not overwrite, clobber, or conflict with user choices set via `harnez agent enable/disable`.
2. Currently, `harnez agent status` requires a specific session ID argument (`harnez agent status <session>`). Users lack a quick, repo-level status overview showing whether `harnez agent` policy is enabled/disabled in the repository, along with a summary of active agents started across agent environments (Codex, Claude, AGY, etc.).

---

## 2. /goal & Acceptance Criteria

### /goal
Add `harnez agent enable` and `harnez agent disable` commands to declaratively configure subagent dispatch mode in `AGENTS.local.md` (and optionally `AGENTS.md` with `--persist`), and enhance `harnez agent status` to display the repository subagent policy state alongside active agents in the repository.

### Acceptance Criteria
1. **CLI Commands (Enable / Disable)**:
   - `harnez agent enable [-d <dir>] [--persist]`: Writes or updates the subagent overlay section in `AGENTS.local.md` (or `AGENTS.md` when `--persist` is given) to configure `subagent_mode: harnez` with standard dispatch guidelines.
   - `harnez agent disable [-d <dir>] [--persist]`: Writes or updates the subagent overlay section to configure `subagent_mode: native` (or removes the harnez policy block).
2. **Repository & Agent Status (`harnez agent status`)**:
   - When invoked without arguments (or with `-d <dir>`), `harnez agent status` displays:
     - **Subagent Policy State**: Enabled (`harnez`), Disabled (`native`), or Unset (default), along with the active overlay source file (`./AGENTS.local.md` or `./AGENTS.md`).
     - **Active Agents Summary**: Table or summary list of active/recent agents running in the repository across agent environments/providers.
   - When invoked with a session ID (`harnez agent status <session>`), continues to display detailed single-session telemetry and state as before.
3. **File & Git Hygiene**:
   - Automatically ensures `AGENTS.local.md` is present in `.git/info/exclude` (via `fsutil.EnsureGitExclude`).
   - Does not clobber existing custom overlay sections (like Concise Mode or custom local developer notes) in `AGENTS.local.md`.
4. **Coordination with `harnez init`**:
   - `harnez init` preserves whatever subagent mode is configured in `AGENTS.local.md`.
   - `harnez init` ensures `Local Overlays` exists in `AGENTS.md` so that the local policy is read by agent sessions.
5. **Unit Tests**:
   - Add test coverage in `cmd/harnez/agent_test.go` and/or `internal/subagent/` testing enable, disable, no-arg status reporting, persistence, and non-destructive overlay updates.
