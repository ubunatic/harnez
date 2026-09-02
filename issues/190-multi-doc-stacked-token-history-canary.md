# 190 — Multi-Doc Git History Token Evolution & Stacked Chart Canary

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Telemetry / Documentation Metrics
**Related**: [[189-git-history-doc-token-time-series-telemetry]], [[057-repo-assessment-and-code-metrics-command]], `internal/assess/dochistory.go`, `scripts/canary-doc-history/`

---

## 1. Problem & Motivation

Issue 189 established the core engine for tracking token and size metrics across the git history of a *single* file (`internal/assess/dochistory.go`).

To understand the aggregate documentation footprint and prompt diet across the entire workspace, we need to analyze **multiple documents simultaneously** (e.g., passing a directory `docs/`, a glob `docs/**/*.md`, or an explicit list of files):
- How has the *total* token weight across all documentation evolved over time?
- What are the largest contributors to context bloat across different time periods (e.g., `docs/lang/` vs `docs/practices/` vs `AGENTS.md`)?
- Can we visualize this in the terminal via a stacked area chart or multi-series timeline?

## 2. Technical Specification

### 1. Multi-Doc History Aggregator (`internal/assess/multidoc.go`)
- **Discovery**: Resolves files from directory paths, glob patterns (`docs/**/*.md`), or argument lists.
- **Concurrent Extraction**: Extracts git commit histories for all resolved files concurrently using the blob-cached git scanner.
- **Unified Timeline Bucketization**:
  - Align individual file snapshots across a unified timeline (e.g., per-commit or sampled weekly/daily timestamps).
  - Compute active file state at each time point (handling files added, removed, or renamed).
  - Calculate total aggregate tokens, bytes, and words, plus per-category/per-file breakdowns.

### 2. Stacked Terminal Visualization & Canary
- Implement standalone canary prototype in `scripts/canary-stacked-doc-history/main.go`:
  - **Aggregate Overview**: Overall sparkline and total token growth across all documents.
  - **Category / File Breakdown Table**: Per-file starting tokens, peak tokens, current tokens, % share of total, and individual sparklines.
  - **Stacked Terminal Visualization**:
    * Stacked ASCII/ANSI proportion bars showing category breakdown over key milestones.
    * Braille/Unicode curve rendering of total cumulative token weight.

## 3. Verification & Empirical Results

1. **Multi-Doc Canary Prototype**:
   - Implemented `internal/assess/multidoc.go` and `scripts/canary-stacked-doc-history/main.go`.
   - Supports directory targets (`docs/`), glob patterns (`docs/lang/*.md`), and file lists (`AGENTS.md`).
   - Supports `--json` structured output and full terminal tabular + stacked proportional bar rendering.
2. **Performance & Latency Verification**:
   - Running 20 files (`docs/lang`, `docs/practices`, `docs/other`, `AGENTS.md`) executes in **61ms** (<100ms requirement).
   - Running the entire `docs/` tree (74 files, 119 commits, 253 unique blobs) executes in **219ms**.
3. **Timeline Unification & Active Presence**:
   - Unit tests in `internal/assess/multidoc_test.go` verified active document presence tracking (file creation, deletion, and modifications).
   - All tests pass (`go test ./...` in 57ms).
4. **Build & Installation**:
   - `make test && make install` completed cleanly.
