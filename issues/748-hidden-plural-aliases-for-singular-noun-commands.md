# 748 — Hidden plural aliases for singular-noun commands

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: CLI Ergonomics
**Related**: 740 (`harnez channel`)

---

/goal Every singular-noun top-level command (and noun subcommands) also accepts its plural as a
hidden alias, covered by one test that walks the command tree; or stop and report when a plural
collides with an existing command and needs a user decision.

## 1. Problem & Motivation
The user typed `harnez channels list` twice and got "unknown command". Plural forms are a natural
guess; the CLI should accept them without listing them in help.

## 2. Technical Specification / Findings
- Cobra `Aliases` show in help; hidden aliases need either a hidden duplicate command or a
  root-level arg rewrite before `Execute`. Pick the simplest that keeps help clean and completion sane.
- Apply generically (walk the tree), not per command, so new nouns get it for free.
- Collisions: `skill` and `skills` are two different commands; `docs` and `issues` are already
  plural. Never shadow an existing command; skip and list collisions in the test.
- Verbs (`apply`, `find`, `read`) need no plural; decide noun-ness by a simple rule (e.g. plural
  `<name>s` not already a command) or an explicit list, documented in `docs/CLIDesign.md`.

## 3. Implementation & Verification Plan
- Unit test: `channels list`, `agents list` resolve; `skills` still runs the skills command;
  plurals absent from `--help`.
- `make install`, then `harnez channels list` works live.
