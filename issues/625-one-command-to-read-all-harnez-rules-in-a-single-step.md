# 625 — One call to read all .harnez rules in a single step

**Status**: Closed — 70cb5ec9 one-call rule read in generated AGENTS.md; 2f442f7e multi-file read skips missing files
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agent Efficiency / Usability
**Related**: #353

---

/goal Agents load all `.harnez/rules` docs in one `harnez read` call, and the generated
instructions show that call; stop and report when blocked on a user decision or denied
permission.

## 1. Problem & Motivation

The user observes agents reading the `.harnez/rules/` docs one file at a time: `Index.md`, then
`Tools.md`, `Issues.md`, `Quota.md`, `Subagents.md`, `Output.md`, `Local.md`. That is 7 tool
calls where one would do. The rules are meant to be read on demand or included, so every session
pays this cost.

## 2. Technical Specification / Findings

- `harnez read` already accepts several files and prints a `=== <path> (N lines) ===` header before
  each, so the mechanics exist. What's missing is an instruction that points agents to it.
- Decision (user, 2026-09-28): no new command. Reuse `harnez read` with several files. The generated
  instruction gives the exact one-liner, listing the files explicitly in Index order, then `Local.md` (a `*.md` glob would
  sort alphabetically and include `Index.md`), which also teaches agents batch reading of whole files in general.
- Check the current `=== <path> ===` header and each file's single `# ` title are enough as
  separators; only change the header format if agents mis-attribute rules.
- Update the generated instruction line ("read `.harnez/rules/Index.md`, then `Local.md`") in the
  managed AGENTS.md/CxxxE.md blocks to the single `harnez read` call.
- Other `See docs/X.md` links: no special form; the same `harnez read a b c` instruction covers them.

## 3. Implementation & Verification Plan

- Test: the generated instruction lists a `harnez read` call that covers every Index-listed file and `Local.md`;
  missing `Local.md` is not an error.
- Regenerate managed blocks via `harnez init -d .`; check one agent session loads rules in one call.

## 4. Delivery

M1 (batched rule-read instruction) delivered in 70cb5ec9.

### M2 — Pre-Work / Required Refinements
- Host check: `harnez read <existing> /nonexistent/Local.md` exits 1, prints no content and dumps usage.
  `Local.md` is git-excluded, so fresh clones break the one-call instruction. With several files,
  `harnez read` must print every readable file, note each missing one on stderr (one line, no usage
  dump), and still exit 0 when at least one file was read. Add a test.
