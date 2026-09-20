# 443 — Polish issues list aliases, terminal output, and completion

**Status**: Open
**Priority**: P2
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `harnez issues` CLI

---

## 1. Problem & Motivation

The `harnez issues` command is awkward to use interactively: the short list alias
is missing, terminal output still exposes an obsolete `Rendered:` test line, and
completion suggests files instead of useful commands, flags, filters, and tickets.

## 2. Goal

Make `harnez issues` discoverable and terminal-friendly: support `issues -l` as an
alias for `issues list`, default interactive `issues list` output to `--text`,
remove the stale `Rendered:` output, and provide useful shell completion.

## 3. Acceptance Criteria

- `harnez issues -l` behaves exactly like `harnez issues list`.
- Interactive `harnez issues list` defaults to text output; explicit output flags
  retain their existing behavior.
- `harnez issues list` no longer prints the obsolete `Rendered:` test line.
- `harnez issues <TAB>` offers issue verbs rather than repository files.
- `harnez issues list <TAB>` offers supported flags and predefined filters.
- `harnez issues list <TAB>` also offers current-repository ticket numbers with
  each ticket title.
- Relevant automated tests cover aliases, output selection, and both completion
  contexts.

## 4. Implementation & Verification Plan

Inspect the Cobra command wiring, terminal-output selection, and completion
registration; update focused tests and documentation/help text as needed. Verify
with the relevant test target and manual completion/help probes.
