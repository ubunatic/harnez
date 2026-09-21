# 480 — Agent prompt input grammar: variadic prompt, -- separator, repeatable -f prompt files, stdin

**Status**: Open

**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: #479 (epic, design §2.2), #481, #483

---

## 1. Problem & Motivation

`agent start` and `agent resume` take exactly one quoted prompt argument
(`cobra.ExactArgs`). Long prompts, prompts assembled from ticket files, and
prompts that begin with `-` or `/` are awkward or impossible to pass safely.

## 2. Technical Specification

- Positional words after the verb form the prompt (joined with single spaces);
  quoting stays optional.
- `--` ends flag parsing; everything after it is appended verbatim, so
  `-- /compact` or `-- --weird` reach the agent unchanged.
- `-f, --file <file>` is repeatable; `-f -` reads stdin once.
- Assembly order, parts joined by one blank line: files in flag order, then
  positional words, then the text after `--`.
- Empty result is an error (`no prompt given`), except for verbs that do not
  take one.
- Stored `start_prompt` holds the user text with files recorded as
  `path (N bytes)`; the harnez protocol preamble is never stored.
- Missing or unreadable prompt files fail before any provider process starts.

## 3. Implementation & Verification Plan

- One `assemblePrompt(files []string, words []string, afterDash []string, stdin io.Reader)`
  helper with table tests: order, blank-line joining, `--` handling, stdin,
  missing file, empty prompt.
- CLI tests for `start` and `resume`, including a prompt starting with `-`.
- Keep the legacy two-argument forms working (see #481 for the compatibility rule).
- Update `agent start --help`/`resume --help` examples and the architecture doc.
