# 385 — Tokens command to count tokens in files and directories

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[057-repo-assessment-and-code-metrics-command]], [[384-harnez-assess-directory-validation-and-d-dir-flag-support]]

---

## 1. Problem & Motivation

Agents and human developers frequently need fast, lightweight inspection of token weights across specific files or subdirectories before feeding them into prompts or context windows (such as assessing context budget for subagents, prompt caching, or skill/docs overhead).

Currently:
1. `harnez assess` is a full repository assessment command producing high-level metrics, health grades, RAMP levels, and track evolutions. It is too heavy when a caller simply wants the token count of specific target files or directories.
2. `internal/assess.EstimateTokens` provides the shared token calculation heuristic (~3.75 chars/token across standard LLM tokenizers), but there is no dedicated, flexible CLI command to run targeted token queries across arbitrary combinations of files and directories with extension filtering.
3. Users and agentic workflows require a fast `harnez tokens` command that accepts multiple file/directory paths, supports inclusion filters (`-I, --include <pattern.ext>`), standard directory rooting (`-d, --dir <path>`), human-readable table output, and machine-readable `--json` output.

## 2. Scope & Design

### 2.1 CLI Syntax & Arguments

```bash
harnez tokens [dirs... | files...] [flags]
```

- **Positional arguments**: Zero or more file paths and/or directory paths. If no positional arguments are provided and `-d/--dir` is not given, default to current working directory (`.`).
- **Flags**:
  - `-I, --include <patterns>`: Filter files matching specific glob or extension pattern (e.g. `*.go`, `*.md`, `go,md` or repeated `-I`).
  - `-d, --dir <path>`: Base working directory / target directory (matching standard Harnez CLI conventions like `harnez find`, `harnez assess`).
  - `--json`: Output full token metrics and file breakdown in JSON format.
  - `-s, --summary`: Output only the summary totals (total files, lines, tokens, bytes) without the detailed per-file table.
  - `-a, --all`: Include hidden files and files normally ignored (e.g. vendor or dotfiles) if applicable.

### 2.2 Core Logic & Package Architecture

- Implement CLI wiring in `cmd/harnez/tokens.go` (and test in `cmd/harnez/tokens_test.go`).
- Reuse existing token calculation and inspection infrastructure in `internal/assess` (`EstimateTokens`, `CountLines`, `InspectFile` / custom walker) or expose a dedicated token counting helper in `internal/assess` or `internal/tokens`.
- Support traversal of directories while respecting inclusion patterns (`--include / -I`) and skipping binaries and oversized files (>10MB) consistently with `internal/assess`.
- Output formatting:
  - Tabular layout with aligned columns: File / Path, Lines, Tokens, Bytes, and aggregate totals.
  - JSON representation containing per-item details and top-level summary totals.

## 3. Exit Criteria

- [ ] `harnez tokens <files...>` prints token, line, and byte counts for the specified files.
- [ ] `harnez tokens <dirs...>` recursively traverses directories and computes token counts.
- [ ] Flag `-I / --include` filters files by glob or extension (e.g. `harnez tokens internal -I *.go`).
- [ ] Flag `-d / --dir` sets the target directory.
- [ ] Flag `--json` prints structured JSON metrics.
- [ ] Unit and integration tests in `cmd/harnez` and `internal/assess` (or `internal/tokens`) verify flag parsing, include filtering, multi-target aggregation, and error handling for nonexistent paths.
