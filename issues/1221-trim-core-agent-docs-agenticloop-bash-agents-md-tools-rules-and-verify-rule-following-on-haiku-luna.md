# 1221 — Trim core agent docs (AgenticLoop, Bash, AGENTS.md, Tools rules) and verify rule following on haiku/luna

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**: 1222, 1223, 1224

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
| `docs/practices/AgenticLoop.md` | 49882 | 22305 |
| `docs/practices/AgenticLoop.lite.md` | 13618 | 9708 |
| `docs/lang/Bash.md` | 10859 | 5483 |
| `docs/lang/Bash.lite.md` | 5123 | 3058 |
| `AGENTS.md` | 11543 | 8991 |
| `.harnez/rules/Tools.md` | 11285 | 5205 |
| `.harnez/rules/` total | ~15000 | 9209 |

Memory files in a Claude session in this repository: 16k tokens before, 4.8k after (`/context`).

- Full and lite AgenticLoop now share one invariant numbering (1-14). Before, the full doc had 13
  and the lite doc 12 with different numbers, so "Invariant 10" and "Invariant 13" references
  pointed at different rules per variant. Cross-references now use rule names.
- Bash section numbers changed; references use the section name ("Directory scoping").
- `Tools.md` no longer repeats `Issues.md`; `Subagents.md` points at the Harnez Agent section.
  The "Harnez Agent" heading and its goal wording stay because
  `TestManagedAgentExamplesParseAgainstCobra` pins them.
- `~/.claude/CLAUDE.md` (owner's global file, outside this repository) had two `@docs/...`
  references that Claude Code inlined into every session. They are plain text now. The project
  doc list is not inlined; the likely reason is the comma after each `@docs/X.md,` entry
  (unverified).
- `/context` works through `harnez agent resume` for Claude sessions; Codex answers it as a
  normal prompt.

## 3. Implementation & Verification Plan

- `make test-q1`: green on the final state. The first run failed on two tests that pin `Tools.md`
  wording; the wording was restored and the tests are unchanged. One golden file
  (`cmd/harnez/testdata/init_solo/AGENTS.md`) follows the new rules header.
- Rule-following sessions: one fixed task (a Bash script, two tracker lookups, a code search, a
  sibling-repo `git` call, the doc-source workflow), scored on 15 observable checks from the
  script, the report and the executed commands. Each session was then asked what it assumed and
  where it read each rule.
- Only the lite variants were exercised by sessions (this repository installs lite). The full
  variants are covered by `TestLiteDocStructuralGate`, `TestSourceBlocksCurrent` and `harnez lint`.

### Results

Checks passed out of 15. "Minor" is `then`/`do` alone on a line, which the owner accepts as a
small error.

| Session | Docs state | Passed | Misses |
|---|---|---|---|
| `haiku:low` | before the trim | 12 | rules read cut at 6000 chars; no `harnez find code`; never opened `docs/Bash.md` |
| `luna:low` | before the trim | 13 | minor; no `harnez find code` |
| `haiku:low` | trim (`fe021790`) | 9 | `set -eu`, tabs, minor; rules read cut at 6000 chars; no `harnez find code`; never opened `docs/Bash.md` |
| `haiku:high` | trim (`fe021790`) | 9 | `set -eu`, tabs, `echo`, minor; no `harnez find code` |
| `haiku:low` | hint fix (`e37b9b2e`) | 9 | `; then`; rules read cut at 3000 chars, so `ls` for tickets (373 instead of 43); no `harnez find code`; never opened `docs/Bash.md` |
| `haiku:high` | hint fix (`e37b9b2e`) | 13 | rules read piped through `head -c 20000` (nothing lost); no `harnez find code` |
| `luna:low` | final (`def50c1c`) | 13 | minor; no `harnez find code` |
| `luna:high` | final (`def50c1c`) | 15 | none |
| `haiku:low` | final (`def50c1c`) | 14 | no `harnez find code` |

What the sessions showed:

- Removing the global inline cost Bash style on haiku until two things were fixed. The Bash line
  in the `AGENTS.md` doc list is the only Bash text a Claude session is sure to see; its old
  wording "break before then/else" was read as "`then` alone on a line". And at 104 lines the
  root `docs/Bash.md` was blocked by the native-read guard (100 lines or more); `haiku:high` gave
  up on the doc after the block. The hint is rewritten and the lite doc is 92 lines.
- haiku pipes the session-start rules read through `head -c N` on its own, also on the old docs.
  The rules header now says to read the whole output; the one `haiku:low` run after that change
  read it in full. One run is a hypothesis, not proof.
- `harnez find code` before `grep`/`rg` was followed by one session in nine, before and after
  the trim (1222).
- haiku:low varies a lot between runs on identical instructions; single runs are weak evidence.

Follow-ups: 1222 (`harnez find code` rule is ignored), 1223 (generated Git hint contradicts Repo
Setup), 1224 (iteration 2: remaining docs, the literal "read `@<file>` links immediately" rule,
the stale Bash hint copy in `internal/subagent/subagent.go`, test-pinned `Tools.md` phrases).
Non-goal: making `harnez lint` flag `do` alone on a line; the owner accepts that slip.

## Decision

- Baseline only on the two low tiers. Nine sessions ran in total instead of the planned six,
  because two regressions on haiku needed a rerun each.
- Global docs stay on the full variant (the `apply` default); they are no longer inlined.
- `Tools.md` keeps every test-pinned phrase; a deeper trim needs those tests reworked (1224).
- The independent reviewer pass was replaced by the test sessions, as agreed in the plan.
