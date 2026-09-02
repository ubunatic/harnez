# 192 — Integrate Git Document History Telemetry into `harnez` CLI Command

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: CLI / Telemetry
**Related**: [[189-git-history-doc-token-time-series-telemetry]], [[190-multi-doc-stacked-token-history-canary]], `cmd/harnez/`, `internal/assess/`

---

## 1. Problem & Motivation

Issues 189 and 190 implemented and validated the single-file and multi-file Git history token evolution engine (`internal/assess/dochistory.go`, `internal/assess/multidoc.go`) via standalone canary scripts (`scripts/canary-doc-history/`, `scripts/canary-stacked-doc-history/`).

To make this capability available across all repositories in daily development, we need to integrate it directly into the `harnez` CLI as a native command or subcommand (e.g., `harnez dochistory [files...]` or `harnez doc-history [files...]` / `harnez stats --doc <path>`).

---

## 2. Technical Specification

### 1. Command Surface
- Add `dochistory` (with alias `doc-history`) to `cmd/harnez/dochistory.go`:
  ```bash
  # Analyze default managed documentation (docs/ and AGENTS.md)
  harnez dochistory

  # Analyze specific file with single-doc timeline and sparkline
  harnez dochistory docs/lang/Go.md

  # Analyze specific directories / glob patterns with stacked category chart
  harnez dochistory docs/lang docs/practices issues
  harnez dochistory "docs/**/*.md"

  # JSON export for external plotting / pipelines
  harnez dochistory --json
  ```
- Flags:
  - `-d, --dir <path>`: target repository directory (defaults to current directory or git root).
  - `--color` / `--no-color`: toggle ANSI colorized stacked bars.
  - `--json`: machine-readable output.

### 2. Implementation
- Wire `cmd/harnez/dochistory.go` directly to:
  - `assess.ExtractDocHistory` for single-file invocations (renders timeline table + 8-step sparkline).
  - `assess.ExtractMultiDocHistory` for directories, globs, or multiple files (renders stacked category bars + doc summary table).
- Register `dochistoryCmd` in `cmd/harnez/main.go`.

---

## 3. Verification Plan

1. Unit tests in `cmd/harnez/dochistory_test.go` verifying flag parsing, repo resolution, and stdout output.
2. Run `harnez dochistory` inside `harnez` and verify default stacked output.
3. Run `harnez dochistory docs/lang/Go.md` and verify single-file timeline table.
4. Verify `--json` output structure.
5. Run `make test` and `make install`.
