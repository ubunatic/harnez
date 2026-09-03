# 221 — Promote Go for Scripts and Tooling Across Repos and Explicitly Demote/Disallow Ad-Hoc Python

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics / Practices
**Related**: [docs/practices/AgenticLoop.md](docs/practices/AgenticLoop.md), [docs/practices/Make.md](docs/practices/Make.md), harnez feedback entry `e7e1c940`

---

## 1. Problem & Motivation

Autonomous coding agents across managed workspaces frequently default to writing ad-hoc Python scripts (e.g. `scripts/audit.py`, `scripts/extract_foo.py`) when asked to perform data extraction, audit tasks, or write helper utilities.

In Go-centric projects (like `ubunatic.com`, `harnez`, `uman`, `lmcoder`, `voxi`), this creates recurring friction:
1. **Toolchain Fragmentation**: Python scripts introduce unmanaged virtual environments, dependency drift (`pip`, `requests`), formatting/linter divergence, and shebang/path portability bugs across developer workstations.
2. **Ecosystem Inconsistency**: The rest of the workspace and tool harnesses are pure Go with zero runtime dependencies.
3. **Loss of Static Safety & Verification**: Go scripts (`scripts/<tool>/main.go` invoked via `go run ./scripts/<tool>`) benefit from static compile-time checking, standard `go test` coverage, strict `gofmt`, and REUSE compliance without extra interpreter setup.

## 2. Proposed Rule & Guidance Updates

1. **Update Shared Harness Guidance (`docs/practices/AgenticLoop.md` / `docs/practices/Languages.md`)**:
   - Add an explicit **Language Hierarchy for Scripts & Tooling**:
     - **Primary Language for Scripts & Tools**: **Go** (`scripts/<name>/main.go` executed with `cd scripts && go run ./<name>`).
     - **Explicit Demotion of Python**: Disallow agents from creating `.py` scripts unless the user or project spec explicitly requires Python (e.g. PyTorch/ML pipelines or Python C-extension bindings).
2. **Standardize Makefile Invocations**:
   - Ensure `Makefile` conventions document `cd scripts && go run ./<tool>` as the standard pattern for repository helper tooling.
3. **Propagate via `harnez apply`**:
   - Bundle this convention into the default agent rules injected across projects during `harnez apply`.

## 3. Implementation & Verification Plan

1. Update `docs/practices/` evergreen files in `harnez` with the Go-first / Python-demoted rule.
2. Run `harnez index` to ensure all issue and doc indices remain consistent.
3. Verify that `harnez apply` propagates the rule into target workspaces.
