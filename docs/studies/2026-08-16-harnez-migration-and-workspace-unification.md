<!-- SPDX-FileCopyrightText: 2026 Uwe Jugel -->
<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# Case Study: Spec Generalization, Harnez Migration & Workspace Diagnostics

*Date: 2026-08-16*  
*Scope: `claudeconfig` → `harnez` rename, Spec-Driven architecture generalization, cross-repo rollout across 20+ projects, and `uman` integration.*

---

## 1. Executive Summary

In a single working session, the project transitioned from a Claude-specific configuration tool (`claudeconfig`) to a cross-agent developer harness (`harnez`), generalized core engineering standards (Spec-driven architecture, Zig, GTK4), mass-upgraded 20+ sibling repositories to the new marker format, and introduced native workspace harness diagnostics into `uman`.

This report evaluates what worked well, details critical near-misses and failure modes encountered during agentic refactoring, assesses final code quality, and measures velocity.

---

## 2. What Worked Well

- **Generalization of Core Principles**:
  - `docs/Spec.md` had previously slipped into the repository as a copy of Cati's internal UI/button layout. Rewriting it into a generic **YAML + JSON Schema** architecture reference fixed a major conceptual flaw for all downstream projects.
  - Zig conventions were enriched with real-world low-level findings (libc `unsetenv` env desync, dynamic C-struct memory layout binding) and registered for automatic signal detection.
- **Autonomous Agent Delegation**:
  - Launching a specialized background subagent to perform mechanical codebase refactoring (Go module path `ubunatic.com/harnez`, entrypoint `cmd/harnez`, Makefile, imports, markers, and docs) allowed rapid parallel execution without context window pollution.
- **Cross-Repo Mass Migration (`make init-siblings`)**:
  - Scripting workspace-wide reconciliation allowed updating 20+ independent repositories in seconds, validating language detection heuristics against diverse stacks (Go, Zig, Rust, Bash, Python/GTK4, Make).
- **Decoupled Architecture (Approaches D + B)**:
  - Rather than coupling `uman` and `harnez` via direct Go package imports, `uman` was equipped with native filesystem diagnostics (`uman doctor`), while `.uman.toml` provided one-command execution (`uman harness-sync`). Both binaries remain completely decoupled and compile independently.

---

## 3. Honest Post-Mortem: Failure Modes & Near-Misses

Autonomous multi-repo automation carries distinct risks. Several real issues occurred and were caught during inspection:

### 3.1 The `CLAUDE.md` Overwrite Hazard (Near-Miss Data Loss)
* **What Happened**: In legacy projects that were never initialized with `harnez` (such as `diagnose` and `proctop`), `CLAUDE.md` existed as a regular file containing custom project rules, but `AGENTS.md` did not exist. `init` wrote a fresh `AGENTS.md` from the blank template and then created a symlink `CLAUDE.md → AGENTS.md`, which silently deleted the original `CLAUDE.md` content from the working tree.
* **How It Was Caught**: Thorough `git diff` audit across all sibling repositories immediately surfaced the deleted sections in `diagnose` and `proctop`.
* **Root Cause Fix**:
  1. Restored the original custom rules in both repositories from git history.
  2. Modified `internal/claude/init.go`: `init` now checks if `CLAUDE.md` is a regular file before creating `AGENTS.md`. If so, it **safely migrates `CLAUDE.md → AGENTS.md` first**, preserving all existing project rules before establishing the symlink.

### 3.2 Incomplete Section Marker Migration
* **What Happened**: The initial marker migration only replaced markers for sections declared in `config.yaml` (`Language Conventions`). Sections created from earlier templates (like `<!-- claudeconfig:begin Project Summary -->`) remained with legacy prefixes in repositories like `cati`, `comfyconf`, `fontwidth`, etc.
* **How It Was Caught**: `uman doctor` flagged 10 warnings across the workspace.
* **Root Cause Fix**: Added `migrateLegacyMarkers` in `init.go` to scan and upgrade all occurrences of `<!-- claudeconfig:` and `# claudeconfig:` across the entire document during project initialization.

### 3.3 Non-Interactive Subshell `$PATH` Resolution
* **What Happened**: When `uman` ran the newly configured `harness-status` command, it failed with `harnez: executable file not found in $PATH` because `~/go/bin` was not in non-interactive environment paths.
* **Root Cause Fix**: Updated `.uman.toml` command execution to explicitly include `PATH="$HOME/go/bin:$PATH"` in subshell invocations.

---

## 4. Code Quality Assessment

| Dimension | Rating | Evidence / Methodology |
|---|---|---|
| **Architecture & Separation** | **A+** | `harnez` owns generation/templates; `uman` owns workspace discovery/diagnostics. Zero shared binary or Go module dependencies. |
| **Idempotency** | **A+** | Re-running `make init-siblings` across 20+ repositories produces `0 changes`. |
| **Backward Compatibility** | **A** | Markdown and Makefile parsers support both new `harnez:` and legacy `claudeconfig:` markers. |
| **Test Coverage & Verification** | **A** | All unit tests (`docs_test.go`, `markdown_test.go`), vet checks, and integration tests pass cleanly; verified with live workspace runs. |
| **Documentation Precision** | **A** | Evergreen docs (`LanguagePipeline.md`, `CLIDesign.md`, `Spec.md`) updated in lockstep with code changes. |

---

## 5. Efficiency & Process Learnings

- **Turnaround Time**: The entire sequence (architecture audit → `Spec.md` rewrite → Zig registration → project rename → mass sibling upgrade → `uman` integration → website updates) was completed in under an hour.
- **The Indispensability of Diff Audits**: When an agent modifies or initializes multiple repositories simultaneously, automated diff inspection across the entire workspace (`for d in ../*; git diff`) is mandatory. Without the post-init diff audit, the `CLAUDE.md` symlink overwrite in `diagnose` and `proctop` would have gone unnoticed until subsequent sessions.
- **Tool Specialization**: Decoupled CLI tools that share common filesystem conventions (like `<!-- harnez:begin -->` and `AGENTS.md`) provide significantly higher resilience and lower maintenance than monolithic multi-repo toolchains.
