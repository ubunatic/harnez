# 402 — Move config diff below status and free top-level harnez diff for visual git diff

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: CLI & Tooling / Context Optimization
**Related**: [[395-harnez-read-command-with-i-image-flag-for-rendering-and-injecting-whole-files-as-visual-context]], [[400-embed-retro-pixel-font-engine-into-internal-readcard-as-default-across-read-docs-cards-init-and-apply]], [[401-default-visual-bounded-card-output-in-harnez-find-and-harnez-issues-with-raw-opt-out]]

---

## 1. Problem & Motivation

During development sprints and pre-commit review gates (Phase 3 of AgenticLoop), reviewer subagents and orchestrators frequently inspect `git diff` or `git show` outputs. When diffs span multiple files or hundreds of lines, printing raw text diffs consumes huge amounts of token quota, risks truncation, and accelerates context window fatigue.

Currently, the top-level `harnez diff` verb is reserved exclusively for global config drift against `~/.claude` (showing what `harnez apply` would change). This causes cognitive friction:
1. Developers and agents naturally expect `harnez diff` to show repository code changes (like `git diff`).
2. Global config drift is more logically a sub-concern of `harnez status` (e.g. `harnez status diff` or `harnez status --diff`).

By moving global config drift inspection under `harnez status diff` (with alias/flag `harnez status --diff`), we **free the top-level `harnez diff` verb**. `harnez diff` can then become the dedicated command for repository/git diffs, rendering dense visual cards by default with clean `--raw` / `--text` / `--json` opt-outs.

---

## 2. Architectural Design

### 2.1 Moving Global Config Drift to `harnez status diff`
- `harnez status diff` (and `harnez status -d / --diff`): Shows what `harnez apply` would change in managed blocks across `~/.claude`, hooks, commands, settings, and docs.
- Preserves all existing flags: `--exit-code` (`-e`), `--capture-docs`, `--out`, `-c`, `-t`.
- Deprecate / migrate the old `harnez diff` config command cleanly.

### 2.2 Top-Level `harnez diff` for Code & Git Diffs
- **Signature**: `harnez diff [flags] [git-ref / files...]`
- **Default Output**: Dense, syntax-highlighted visual card (`-I`) using retro pixel fonts (`Font5x8`), bounded within ViT dimensions, with column wrapping.
- **Flags Supported**:
  - `--cached` / `--staged`: Show staged changes.
  - `--raw` / `--text`: Opt out of visual cards and print standard unified diff text.
  - `--json`: Emit structured JSON metadata with hunks, affected files, and token stats.
  - `-c, --columns`: Column count for visual layout (1-4, auto).
  - `--font`, `--font-size`, `--theme`, `--tokens`.
- **Pipeline & Non-TTY Detection**:
  - When stdout is piped to a file or Unix command, automatically defaults to text output unless `-I` is explicitly passed.

### 2.3 Syntax Highlighting in `internal/readcard/lexer.go`
- Add unified diff lexer to colorize additions (`+` green), deletions (`-` red), hunk headers (`@@` cyan), and file metadata headers.

---

## 3. Acceptance Criteria

- [ ] Move global apply drift command to `harnez status diff` (and support `harnez status --diff`).
- [ ] Implement top-level `harnez diff` wrapping git diff with visual card default.
- [ ] Support `--cached`, `--staged`, `--raw`, `--text`, `--json`, and file/ref arguments.
- [ ] Add unified diff syntax highlighting in `internal/readcard/lexer.go`.
- [ ] Auto-fallback to text mode when stdout is non-TTY / piped.
- [ ] Add unit and CLI tests in `cmd/harnez/diff_test.go` and `cmd/harnez/status_test.go`.
- [ ] Update documentation (`docs/CLIDesign.md`, `AGENTS.md`, `docs/practices/AgenticLoop.md`).
