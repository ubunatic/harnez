# 276 — Add harnez lint command for fast rule adherence checking

**Status**: Closed — resolved in 5d09901
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature / Agentic Ergonomics
**Related**: `cmd/harnez/`; `internal/lint/`; `docs/lang/Bash.md`; `docs/lang/Go.md`; `AGENTS.md`; Issue 275

---

## 1. Problem & Motivation

While repository conventions and rules are documented in `docs/lang/*.md` and summarized in `AGENTS.md`, agents and developers currently lack an immediate, automated CLI command to verify style and rule adherence on individual files or changed buffers before committing.

Common style regressions (such as introducing `if [ ... ]` or `; then` in Bash scripts, incorrect terminal display width assumptions in Go, or broken harnez section markers in Markdown/Makefiles) are currently only discovered during manual review or pre-commit checks.

Adding a fast, standalone `harnez lint` CLI command provides an immediate feedback loop during TDD and sprint reviews, allowing agents and human developers to self-correct rule violations immediately. Structuring the linter with a modular, rule-based architecture allows progressive addition of language-specific checkers and potential `--fix` capabilities.

## 2. Scope

**In scope:**
- Implement `harnez lint [flags] <files...>` CLI subcommand in `cmd/harnez/`.
- Flag support:
  - `--lang <auto|bash|go|markdown|make>` (default `auto`, inferred from file extension or shebang).
  - `--check` / non-zero exit code when rule violations are found.
  - `--json` for machine-readable output of lint findings (file, line, column, rule ID, message).
- Core initial rule checks:
  - **Bash**: Detect `[` / `[[` test brackets, detect `; then` / `; do` on the same line, flag `.` when used for sourcing instead of `source` where applicable, verify `set -euo pipefail`.
  - **Go**: Flag basic style checks and pitfalls per `docs/lang/Go.md` (e.g. display width / rune indexing pitfalls, non-idiomatic patterns).
  - **Markdown / Managed Docs**: Verify integrity of harnez section markers (`<!-- harnez:begin ... -->` / `<!-- harnez:end ... -->`) and bundled frontmatter.
- Modular package in `internal/lint/` with clean extensible interfaces (`Rule`, `Finding`, `Linter`) for future language additions and auto-fix extensions.
- Comprehensive unit tests covering CLI flags, rule matchers, output formatting, and exit codes.

**Out of scope:**
- Replacing general-purpose external linters (e.g. `golangci-lint`, `shellcheck`); `harnez lint` focuses specifically on harnez-managed project rules, conventions, and invariants.
- Full automatic AST refactoring in the initial version (structure rule design to support future `--fix`).

## 3. Acceptance Criteria

- [ ] `harnez lint <file>` inspects the specified file(s) and reports violations with line numbers and explanations.
- [ ] CLI supports `--lang` override, `--json` formatting, and non-zero exit code on failure with `--check`.
- [ ] Initial Bash rules catch `[ ... ]`, `[[ ... ]]`, `; then`, and ensure `if test` / 3-line conditionals compliance.
- [ ] Initial marker/doc rules catch mismatched or corrupted `harnez:begin` / `harnez:end` markers.
- [ ] `internal/lint/` package is modular and easily extensible for new rules and language targets.
- [ ] Comprehensive unit tests verify lint execution, CLI flags, JSON output, and exit status.
