# 1068 — Adopt plain mode for general tasks and create role profiles to pin docs, skills, and tools

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [1027](1027-agent-context-modes-plain-no-skills-role-skills-for-pi-and-small-window-models.md), [1057](1057-epic-plain-agent-runs-and-temporary-model-selection.md), [926](926-support-installing-custom-repository-provided-skills-into-agent-environments.md), [1034](1034-local-model-runs-attach-all-input-as-files-and-give-tight-role-guidance-so-the-agent-reads-nothing-itself.md)

---

## 1. Problem & Motivation

Issue 1027 introduced selectable agent context modes (`default`, `no-skills`, and `plain`) in `spec/agent.yaml` and the subagent runner (`--context-mode`). While `plain` mode successfully strips all context files, skills, MCP servers, and restricts tool exposure to the core coding set `[read, edit, write, grep, find, bash]`, it is currently a fixed, one-size-fits-all definition.

In practice, general agent tasks require distinct contextual and tooling configurations depending on their specific role:
- **Reviewers**: Do not need write/edit tools, and benefit from read-only tools plus review-specific guidance docs.
- **Web Researchers / Explorers**: Require web search or browser inspection tools, but do not require code editing or repository-wide skills.
- **Coders / Mechanical Implementers**: Need exact file mutation and build tools, but do not need extraneous orchestrator or communication skills.

Currently, there is no declarative way for users (globally in `~/.config/harnez/`) or repositories (locally in `.harnez/repo.yaml` or `.harnez/profiles/`) to define **role profiles** that compose context modes, pinning specific subsets of documentation, skills, and tools to specific agent roles.

## 2. Technical Specification & Design

1. **Role Profiles Schema (`spec/agent.yaml` & `.harnez/repo.yaml`)**:
   Define a structured profile schema specifying:
   - `role`: Target agent role (`developer`, `reviewer`, `advisor`, `searcher`, etc.).
   - `context_mode`: Baseline context mode (`plain`, `no-skills`, `default`).
   - `docs`: List of specific documentation files or topic cards pinned to the role (e.g. `[docs/ReviewLearnings.md]` for reviewers).
   - `skills`: Pinned skills allowed or injected for this role.
   - `tools`: Exact whitelist of tool names enabled for the role (overriding or refining the baseline mode).

2. **Resolution & Cascading Hierarchy**:
   Profiles resolve with clear precedence:
   - Command-line flags (`--context-mode`, `--tools`, `--role`) override profile defaults.
   - Repository-local profiles (`.harnez/profiles/<role>.yaml` or `.harnez/repo.yaml: profiles`) take precedence over global profiles.
   - User global profiles (`~/.config/harnez/profiles/<role>.yaml` or `~/.config/harnez/local.yaml`).
   - Default builtin role profiles in `spec/agent.yaml`.

3. **Subagent Runner Integration**:
   - When `harnez agent start --role <role>` is invoked, the driver automatically applies the corresponding role profile if `--context-mode` is not explicitly set, passing the resolved tools whitelist, pinned docs, and skill filters down to the provider runner (Pi, Claude, AGY).

## 3. Goal & Acceptance Plan

/goal Implement declarative role profiles (global and repo-local) to pin docs, skills, and tools to agent roles on top of plain context modes, verified with configuration resolution tests and subagent invocation unit tests, or stop and report when blocked on user input or denied permission.

1. **Schema & Builtin Profiles**: Extend `spec/agent.yaml` and JSON schema to support `role_profiles` with `docs`, `skills`, `tools`, and `context_mode`.
2. **Profile Resolution Engine**: Implement cascading lookup (`internal/agentpolicy` or `internal/subagent`) supporting global (`~/.config/harnez`) and repo-local (`.harnez/`) definitions.
3. **CLI & Driver Plumbing**: Plumb profile resolution into `harnez agent start` and `piRunArgs`.
4. **Verification**:
   - Unit tests covering profile inheritance, override precedence, and invalid tool/doc definitions.
   - Integration test verifying a reviewer role runs in plain mode with read-only tools and pinned review docs.
