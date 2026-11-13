---
kind: topic
---

# Trimming the core agent docs and turning the run into a skill (2026-11-13)

Tickets: 1221 (the trim), 1225 (the `/harnez-cleanup` skill), follow-ups 1222, 1223, 1224.
Host: one Claude Code session, Haiku 5.5 at the start, Fable 5.2 from the test phase on.

## 1. Context

A session in this repository loaded 16k tokens of memory files before the first turn. The owner
asked for a refinement of the core docs (AgenticLoop, Bash, `AGENTS.md`, the generated rule
files) under three rules: assume capable agents, keep the owner's own rules, drop what
`harnez <verb> --help` prints. Cheap models had to keep following the rules, so each state was
tested with `harnez agent` sessions on `haiku` and `luna` at `low` and `high` effort. Afterwards
the owner asked for the method as a skill, tested by a `terra:high` agent on one existing skill.

## 2. Outcome

- Memory files in a Claude session here: 16k -> 4.8k tokens (`/context`). 10.3k of the 16k came
  from two `@docs/...` lines in the owner's global instruction file that inlined stale doc copies.
- Sizes in bytes: AgenticLoop 49882 -> 22305 (lite 13618 -> 9708), Bash 10859 -> 5483 (lite
  5123 -> 3058), `AGENTS.md` 11543 -> 8991, `.harnez/rules/` about 15000 -> 9209.
- Rule following, checks passed out of 15 on one fixed task, one run per cell (misses per
  session are in ticket 1221):

| Model | Old docs | First trim | Hint fix | Final |
|---|---|---|---|---|
| `haiku:low` | 12 | 9 | 9 | 14 |
| `haiku:high` | - | 9 | 13 | - |
| `luna:low` | 13 | - | - | 13 |
| `luna:high` | - | - | - | 15 |

- The method is the `/harnez-cleanup` skill (`docs/commands/HarnezCleanup.md`, 80 lines).
- Skill verification (1225): a `terra:high` session in the `orchestrator` role followed the
  skill for the `harnez-init` skill in 16 minutes, with four `luna` test sessions and one
  developer helper. `docs/commands/HarnezInit.md` went from 59 lines, 3818 bytes to 28 lines,
  1612 bytes. The sessions scored 5/5 before and 5/5 and 4/4 after, so the run shows no loss; the
  checks were too easy to show more.
- Four tests that asserted sentences now assert commands, tool names and structure (1224).

## 3. What Went Wrong

- **The first trim made haiku worse (12 -> 9).** The inlined global Bash doc had been the only
  Bash text haiku was sure to see. Without it, the one-line Bash hint in `AGENTS.md` carried the
  style, and its wording "break before then/else" was read as "`then` alone on a line". Caught
  by the rerun; fixed by rewriting the hint.
- **A 104-line doc was unreadable.** The native-read guard blocks files of 100 lines or more;
  `haiku:high` gave up on `docs/Bash.md` after the block. Caught in the session's command log;
  the lite doc is 92 lines now.
- **haiku truncates the session-start rules read on its own** (`head -c 3000` to `20000`), on
  the old docs as well. One run then counted 373 tickets instead of 43. The rules header now
  says to read the whole output; one run confirmed it, which is not proof.
- **Tests that assert sentences failed twice.** After the trim two tests failed on `Tools.md`
  wording, and the wording was restored. In the skill run the helper's 28-line rewrite failed a
  test with 22 literal phrases; the helper put six headings back and reworded sentences to
  match ("If NOT initialized, Stop immediately"), 45 lines that no session had tested. The
  tests were loosened afterwards and the tested 28-line version restored.
- **A wrong explanation stood for a while:** "no session read the `@docs/...` links" was first
  taken as agents ignoring a rule. A canary showed the cause: the doc-list renderer writes a
  comma after each `@docs/X.md`, and Claude Code imports only a clean `@path`.
- **The skill run's test sessions were not comparable.** Each built its own fixture, wrote its
  own transcript, and one rerun had nothing to commit. The skill now tells the runner to prepare
  fixtures and to score from the harness's command log.

## 4. Learnings

- Measure what loads before judging what is written. The biggest saving was a configuration
  fault, not prose.
- Take a baseline before the first edit. Without it the 9 of 15 would have passed as normal
  haiku behaviour.
- Score from artifacts and executed commands, then ask the session which rules it read and
  which it set aside. The follow-up question found 1223 and the `@<file>` problem.
- A rule that is present is not a rule that is followed: `harnez find code` before `grep` was in
  `Tools.md` throughout and one session in nine did it (1222).
- Delivery decides more than wording on the cheapest tier: the same Bash rules gave a clean
  script inlined and a poor one when the agent had to fetch them.
- Single `haiku:low` runs vary too much to settle anything; repeat when a result decides.
- Tests should assert commands and structure. A test that asserts sentences makes agents bend
  the text to the test.
- A commanded cleanup works in the `orchestrator` role; no role that both edits and spawns was
  needed.
- Checks that every session passes detect nothing. Pick checks the current text sometimes
  fails.
- A fresh reviewer on the loosened tests was worth its 42 seconds: it found that the rewritten
  command test could pass on a YAML comment alone, and that dropping every phrase had also
  dropped the consent rule for `index_repository`.

## 5. Follow-Up Tickets

- 1222: agents skip `harnez find code`; measure its benefit before keeping the rule.
- 1223: the generated Git hint contradicts the protected-main Repo Setup.
- 1224: iteration 2: per-agent rule delivery (`harnez read --rules`), the renderer comma as a
  per-doc choice, the remaining copyable docs, the other skill tests that assert phrases.
- 1225: the skill and its verification run (closed with this study).
- Non-goal: `harnez lint` flagging `do` alone on a line; the owner accepts that slip.
