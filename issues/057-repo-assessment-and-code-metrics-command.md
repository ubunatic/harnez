# 057 — `harnez assess`: Fast Code/Doc Metrics & Repo Feasibility Report

**Status**: Closed — resolved in implementation of `harnez assess`
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md), [docs/Bash.md](../docs/lang/Bash.md), [docs/Go.md](../docs/lang/Go.md)

---

## 1. Problem & Motivation

When human developers or AI agents first inspect a repository, directory, or individual file (or before committing changes), they need an immediate assessment of "basic feasibility":
- How large is the project in terms of code vs. documentation?
- What are the primary languages and file compositions?
- Are there maintenance red flags or code smells (e.g. monolithic files, oversized Bash scripts, missing tests, or bloated documentation)?
- How many tokens of context would be consumed if an agent needs to ingest key modules?

Currently, developers and agents must run fragmented ad-hoc commands (`find`, `wc -l`, `cloc`, custom scripts), which are inconsistent across environments, slow, and produce unstructured output not tailored for fast cognitive digestion or pre-commit gates.

---

## 2. Goals & Key Requirements

1. **Polyglot & Harnez-Aware File Discovery**:
   - Classify files into Code, Docs (including Harnez-managed docs: `AGENTS.md`, `CLAUDE.md`, `docs/`), Configs, and Tests.
   - Ignore build artifacts, `.git`, `vendor/`, `node_modules/`, and binary files.
   - Respect `.gitignore` and common ignore patterns.

2. **Metrics & Token-Aware Sizing**:
   - **Files**: Total count per language/category.
   - **LOC (Lines of Code)**: Non-blank, non-trivial lines.
   - **TOC (Tokens of Code/Content)**: Estimated token count (e.g. ~3.5–4.0 chars/token or fast BPE-like heuristic) to give agents and developers an accurate understanding of context window weight.
   - **Words & Documentation Ratios**: Code-to-Doc ratio, Code-to-Test ratio.

3. **Simple Metrics & Heuristic Warnings**:
   - **High LOC / TOC warnings**: Highlight oversized monolithic files (e.g. single code file > 500 LOC or > 3,000 tokens).
   - **Bash Script Warning Thresholds**: Bash is inherently harder to maintain at scale; trigger warnings for shell scripts exceeding 100–150 LOC (encouraging modularization or rewrite in Go/Python per `docs/Bash.md`).
   - **Missing / Skewed Test Coverage**: Warn if major code directories lack corresponding test files.
   - **Stale / Oversized Harnez Assets**: Flag bloated `AGENTS.md` or unindexed docs.

4. **Concise Feasibility Report (3–20 Lines)**:
   - Compact output formatted for instant human reading and token-efficient agent ingestion:
     - Single file: 3–5 lines (LOC, TOC, warnings, language).
     - Subdirectory: 5–10 lines (breakdown, top files, health flags).
     - Full Repo: 10–20 lines (category summary, top languages, health & feasibility score, key warnings).
   - `--json` flag for machine readability and tooling integration.

5. **Pre-Commit Integration**:
   - Non-blocking advisory check in pre-commit workflows / hooks.
   - Fast execution (< 50ms) to ensure zero friction during commit loops.

---

## 3. Analysis: External Quality Tools vs. Native Engine

### Candidate External Tools Evaluated

| Tool | Language | Pros | Cons / Constraints |
|---|---|---|---|
| **`scc`** (Sloc, Cloc and Code) | Go | Extremely fast, calculates LOC, blank, comments, cyclomatic complexity, estimated COCOMO costs. Written in Go. | External dependency if invoked as CLI; library import adds non-trivial dependency tree. |
| **`tokei`** | Rust | Very fast, accurate language parsing. | Requires Rust binary installed on host; not universally present. |
| **`cloc`** | Perl | Widely available on Linux. | Relatively slow on medium/large repos, external Perl dependency. |
| **`shellcheck`** | Haskell | Gold standard for Bash linting and quality. | External binary; should be used opportunistically when present. |
| **`gocyclo` / `gocognit`** | Go | Measures cyclomatic / cognitive complexity for Go. | Go-specific; requires binary or AST parsing. |

### Recommended Architectural Approach

1. **Primary: Native Go Zero-Dependency Engine in `harnez`**:
   - Implement native file scanner, fast non-whitespace line counter, and token estimation heuristic inside `internal/assess/` (or `internal/metrics/`).
   - Guarantees zero external binary dependencies and sub-50ms execution speed across Linux, macOS, and container environments.
2. **Secondary: Opportunistic Tool Enrichment**:
   - If external tools (e.g. `shellcheck`, `scc`, `git`) are detected in `$PATH`, optionally incorporate high-signal diagnostics (e.g. ShellCheck severity warnings) without blocking the command if absent.

---

## 4. Proposed CLI UX

```bash
# Assess current working directory (or repository)
harnez assess

# Assess specific file or path
harnez assess ./scripts/smoke-test.sh
harnez assess ./internal/usage/

# Pre-commit non-blocking check
harnez assess --precommit

# Machine-readable JSON output
harnez assess --json
```

### Example Output (Repo / Directory)

```text
── Repository Assessment: ubunatic/harnez ──────────────────────────────
Category      Files    Lines (code)    Tokens (est)    Health
Code (Go)        18           3,420         28,500    ✓ Healthy
Code (Bash)       3             142          1,150    ✓ Clean (<150 LOC/file)
Docs             12           1,850         14,200    ✓ Well-documented
Tests            14           2,100         17,400    ✓ Test ratio: 61%
────────────────────────────────────────────────────────────────────────
Feasibility: High (modular, well-tested, within agent context budget)
Warnings:
  • none
```

---

## 5. Implementation & Verification Plan

1. **Package `internal/assess`**:
   - File walker with ignore filters (`.git`, binary detection).
   - Classifiers: Code (by extension), Docs (`.md`, `.rst`, `.txt`), Config (`.yaml`, `.json`), Tests (`*_test.*`, `test_*.*`).
   - Metrics accumulator: LOC (non-whitespace), TOC (token approximation), file size.
   - Warning rules: High LOC, high Bash LOC, low test ratio, oversized docs.
2. **CLI Command `harnez assess` in `cmd/harnez/main.go`**:
   - Wire flags: `--json`, `--warn-only`, path argument.
3. **Unit & Integration Tests**:
   - Test against fixtures with varying file types, high-LOC bash scripts, and doc collections.
   - Benchmark execution time (< 50ms on typical repo).
4. **Docs & Pre-commit Hook Scaffolding**:
   - Update `docs/CLIDesign.md` and Makefile templates for pre-commit check invocation.
