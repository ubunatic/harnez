# 1221 — Trim core agent docs (AgenticLoop, Bash, AGENTS.md, Tools rules) and verify rule following on haiku/luna

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**:

---

## 1. Problem & Motivation

The core agent docs restate normal engineering practice and repeat what `harnez <verb> --help`
prints. A session in this repository loaded 16k tokens of memory files before the first turn;
10.3k of that came from two global docs that `~/.claude/CLAUDE.md` inlined through `@docs/...`
references. A `haiku:low` test session cut the 15 KB rules read off at 6000 characters by itself.

Goal: keep the owner's own rules (Bash house style, harnez commands, invariants), drop the rest,
and check that cheap models still follow the rules.

## 2. Technical Specification / Findings

Iteration 1 sizes (bytes):

| File | Before | After |
|---|---|---|
| `docs/practices/AgenticLoop.md` | 49882 | 22299 |
| `docs/practices/AgenticLoop.lite.md` | 13618 | 9702 |
| `docs/lang/Bash.md` | 10859 | 5478 |
| `docs/lang/Bash.lite.md` | 5123 | 3009 |
| `AGENTS.md` | 11543 | 8721 |
| `.harnez/rules/Tools.md` | 11285 | 5193 |
| `.harnez/rules/` total | ~15000 | 9197 |

- Full and lite AgenticLoop now share one invariant numbering (1-14). Before, the full doc had 13
  and the lite doc 12 with different numbers, so "Invariant 10" and "Invariant 13" references
  pointed at different rules per variant. Cross-references now use rule names.
- Bash section numbers changed; references use the section name ("Directory scoping").
- `Tools.md` no longer repeats `Issues.md`; `Subagents.md` points at the Harnez Agent section.
  The "Harnez Agent" heading and its goal wording stay because
  `TestManagedAgentExamplesParseAgainstCobra` pins them.
- `~/.claude/CLAUDE.md` (owner's global file, outside this repository) had two `@docs/...`
  references that Claude Code inlined into every session. They are plain text now. The project
  doc list does not inline because each `@docs/X.md,` entry ends in a comma.
- `/context` works through `harnez agent resume` for Claude sessions; Codex answers it as a
  normal prompt.

## 3. Implementation & Verification Plan

- `make test-q1`: green after the trim (first run failed on two tests that pin `Tools.md`
  wording; the wording was restored, the tests are unchanged).
- Rule-following sessions: one fixed task (a Bash script, two tracker lookups, a code search, a
  sibling-repo `git` call, the doc-source workflow), scored on 15 observable checks from the
  script, the report and the executed commands. Baseline on the old docs: `haiku:low` and
  `luna:low`. After the trim: `haiku:low`, `haiku:high`, `luna:low`, `luna:high`. Each session is
  then asked what it assumed and where it read each rule.
- Results are recorded below when the sessions finish.

## Decision

- Baseline only on the two low tiers; six sessions are enough for one iteration.
- Global docs stay on the full variant (the `apply` default); they are no longer inlined.
- `Tools.md` keeps every test-pinned phrase; a deeper trim needs those tests reworked.
