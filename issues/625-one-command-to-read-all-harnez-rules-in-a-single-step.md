# 625 — One command to read all .harnez rules in a single step

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agent Efficiency / Usability
**Related**: #353

---

/goal Agents load all `.harnez/rules` docs (Index order, then `Local.md`) with one command, and the
generated instructions tell them to; stop and report when blocked on a user decision or denied
permission.

## 1. Problem & Motivation

The user observes agents reading the `.harnez/rules/` docs one file at a time: `Index.md`, then
`Tools.md`, `Issues.md`, `Quota.md`, `Subagents.md`, `Output.md`, `Local.md`. That is 7 tool
calls where one would do. The rules are meant to be read on demand or included, so every session
pays this cost.

## 2. Technical Specification / Findings

- `harnez read` already accepts several files and prints a `=== <path> (N lines) ===` header before
  each, so the mechanics exist. What's missing is a single entry point that knows the file set and
  order, and an instruction that points agents to it.
- Proposal: `harnez rules` (or `harnez read --rules`; choose one, check `docs/CLIDesign.md`) prints
  the files listed in `Index.md`, in order, then `Local.md`, concatenated.
- Separator: each rule file has exactly one `# ` title, so the title can serve as the separator;
  prefix a one-line path marker (or a small front-matter/comment line) so agents can cite the
  source file. Keep the output plain Markdown.
- Update the generated instruction line ("read `.harnez/rules/Index.md`, then `Local.md`") in the
  managed AGENTS.md/CxxxE.md blocks to "run `harnez rules`", with the file-by-file path as fallback.
- Open: should other on-demand docs (e.g. `See docs/X.md` links) get the same batch form?

## 3. Implementation & Verification Plan

- Test: output contains every Index-listed file in order plus `Local.md`, each with its marker;
  missing `Local.md` is not an error.
- Regenerate managed blocks via `harnez init -d .`; check one agent session loads rules in one call.
