---
kind: story
---

# Story: trimming the agent docs and making it a skill (2026-11-13)

Previous story: [2026-11-11 the last Jules sweep](2026-11-11-jules-final-sweep-story.md).
This story covers one host session only, not everything since that story.
Sub-study with the measurements: [core docs trim and cleanup skill](2026-11-13-core-docs-trim-and-cleanup-skill.md).

Review: second-vendor reviewer `codex:terra:high` (gpt-6.1-terra) checked numbers, commits, ticket
states and the post-mortem against git and the tracker. Five findings, all fixed: "no lost check"
overstated the skill run, two learnings contradicted the delivered skill, the missing command-log
verb was stated too firmly, ticket 1226 lies outside the committed range, and the test verdict was
too broad. Session timing, tool-call and prompt counts and the split of commits between host,
helper and owner come from the host transcript and were not checkable; git shows one author.

## 1. Header & Context

- **Session:** Claude Code session `6566b0ff`, 2026-11-13 01:36 to 11:02 CET, about 9.4 hours wall
  clock. The transcript has three gaps of 7.2 hours in total (02:30 to 06:30, 06:35 to 07:27,
  07:27 to 09:48), so about 2.2 hours were active. The first gap follows the owner's `@<file>`
  question; its cause was not checked. Host model: Claude Fable 5.2 at high effort after nine
  opening messages on Haiku 5.5. One context compaction. Baseline commit `4e2ffb28`.
- **Goal:** the owner saw "Memory files: 16k tokens (8.0%)" at session start and asked for a
  refinement of the core agent docs (AgenticLoop, Bash, `AGENTS.md`, the generated rule files):
  assume capable agents, keep the owner's own rules, drop what `harnez <verb> --help` prints, and
  test rule following on `haiku` and `luna` at `low` and `high` effort. Later goals: a
  `/harnez-cleanup <scope>` skill that repeats the method, tested by a `terra:high` agent, and
  looser tests where they assert sentences.
- **Authority:** `/decide` (push and self-merge after tests and review); from 09:56 the owner
  allowed direct commits and pushes to `main`. No release.

## 2. Executive Summary

Memory files in a Claude session in this repository went from 16k to 4.8k tokens. Most of that was
a configuration fault: two `@docs/...` lines in the owner's global instruction file inlined 10.3k
tokens of stale doc copies. The full AgenticLoop, the full Bash doc and `Tools.md` shrank by about
half, the lite variants by 29 and 40 percent, `AGENTS.md` by 22 (bytes in ticket 1221). Nine test
sessions on one fixed
task showed the first trim made `haiku` worse (12 to 9 of 15 checks) and two fixes brought it back
(14); `luna` held 13 and reached 15 at high effort. PR 164 was merged by the owner.

The method became the `/harnez-cleanup` skill. A `terra:high` orchestrator followed it for the
`harnez-init` skill in 16 minutes and cut that skill from 59 to 28 lines. Its four test sessions
scored 5 of 5 before, and 5 of 5 and 4 of 4 (one check not applicable) after; with unequal
fixtures and self-written transcripts that shows nothing obvious broke, not parity. Four tests that asserted sentences now assert commands, tool names and structure. Tickets 1221 and 1225
are closed; 1222, 1223, 1224 and 1226 are open.

## 3. What Worked Well

- **Measuring before editing.** `/context` named the two inlined global docs within minutes; no
  amount of prose trimming would have saved as much.
- **A fixed task with observable checks.** Fifteen checks, scored from the script, the report and
  the executed commands, turned "the docs feel shorter" into a number per model and state.
- **Asking the sessions afterwards.** "Which rules did you read, where, and which did you set
  aside" found the contradicting Git hint (1223) and the unread `@docs/...` links.
- **A canary before a theory.** Four `@path` forms in a scratch project settled in one session why
  the doc list is never inlined: the renderer's trailing comma.
- **Delegation through the ticket.** The `terra:high` run needed one prompt line pointing at ticket
  1225; the orchestrator role, one developer helper and four test sessions did the rest, and the
  run reported its own limits plainly.
- **The reviewer pass.** A 42-second read-only review by `codex:sol:low` found two real defects in
  the host's test rewrite.

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)

- **The first trim was a regression on haiku.** Removing the global inline removed the only Bash
  text haiku was sure to see. The remaining one-line hint said "break before then/else" and was
  read as "`then` alone on a line"; the 104-line `docs/Bash.md` was blocked by the read guard.
  Caught by the first rerun; fixed by a rewritten hint and a 92-line lite doc.
- **Two tests failed after the trim** because they asserted exact `Tools.md` sentences. The host
  restored the wording and moved on; the real fix (loosen the tests) came hours later, after the
  owner called the tests too rigid.
- **A wrong explanation reached the owner.** The host reported "no session read the `@<file>`
  links" as agents setting a rule aside. The owner objected that `@<file>` forces Claude to read.
  The canary showed the owner was right about the mechanism and that harnez disables it by
  accident.
- **The skill run tested the wrong version.** The helper's 28-line rewrite failed a test with 22
  literal phrases, so it bent the text back to 45 lines that no session had tested. The host's task
  spec had forbidden test edits and capped the sessions at four, which left no retest. Fixed after
  the run by loosening the test and restoring the tested version.
- **The skill's checks were too easy.** All baseline sessions scored 5 of 5, so the run cannot show
  that the shorter skill is as good, only that nothing obvious broke. Fixtures also differed per
  session, and the sessions wrote their own transcripts (1226). One after-run first failed on an
  invalid fixture it had built itself, the other had nothing to commit, and both after-runs called
  sample links broken that a before-run had recognised as fenced examples; whether that is a
  regression of the shorter skill was not settled.
- **The host's first test rewrite could pass on a YAML comment alone,** and dropping every phrase
  had dropped the only check on the `index_repository` consent rule. Caught by the reviewer, not by
  the host or the green test run.
- **A reversed sentence in the new skill** ("a baseline with full marks cannot show a loss") was
  also caught by the reviewer.
- **Host rule slips:** the skill is 82 lines against the 60 the host announced; the host edited
  repository files from Bash three times in the last phase although a hook asks for the edit tools;
  the studies index row was regenerated after the last test run, so that one generated line is
  untested.
- **Weak evidence stays weak.** Every score is a single run and `haiku:low` varies between runs.
  `haiku:high` was not rerun on the final state; the full doc variants were never exercised by a
  session.

## 5. Quality & Invariants Audit

| Aspect | Verdict | Evidence |
|---|---|---|
| Architecture & module separation | Stable | No non-test Go changed. Docs were edited at their sources (`docs/lang/`, `docs/practices/`, `config.yaml`); root copies follow through `harnez init`. |
| Idempotency | Stable, one drift found | `harnez init --variant lite` after the merge rewrote only `docs/Search.md`, which had still been the full variant; the owner committed it (`c11fa8c2`). |
| Backward compatibility | Changed on purpose | Invariant and Bash section numbers changed; references use rule and section names now. Both AgenticLoop variants share one numbering (1-14). |
| Test coverage & verification | Diverged slightly, by decision | `make test-q1` green on the last code state (`1ebc5fc3`); this story, ticket 1226 and two index rows came after it and are docs only. Four tests check names instead of sentences; unit tests no longer notice when a trim drops five agent rules (listed in 1224). Rule following is checked by sessions, single runs. |
| Lint | Not compared | `harnez lint` reports 241 warnings, all Go structure rules; no non-test Go changed, and no count was taken at the baseline. |

Overall: the docs improved and the evidence for that is thin but honest; test rigor went down on
wording by the owner's decision and up on one point (every `harnez agent` example in `config.yaml`
is parsed, not only one section).

## 6. Efficiency & Velocity Assessment

- **Output:** 16 commits since `4e2ffb28`: 11 by the host, 3 by the `terra:high` helper, 1 by the
  owner, 1 merge. Two of the 15 non-merge commits touch Go, tests only. Outside `issues/`: 20
  files, 1177 lines added, 1581 removed; 4 of those files are tests (55 added, 63 removed), the
  rest docs, config and generated copies. Tickets: 5 opened in those commits (1221 to 1225) and
  1226 with this story; 2 closed. No release.
- **Agent sessions:** 9 rule-following sessions, 1 canary, 1 orchestrator with 4 test sessions
  and 1 developer helper, 1 reviewer: 17, counted from the tickets and this session's records,
  plus the reviewer of this story.
- **Host effort:** about 230 tool calls, four fifths of them Bash, in about 2.2 active hours. The
  owner sent 13 prompts and ran two shell command lines.
- **Owner corrections:** three, all on substance: accept `then` alone on a line as a small error,
  the objection about `@<file>`, and "the tests seem too rigid". One wording question ("who should
  pin a skill onto what?") came from host jargon.
- **Compared with the previous story:** smoother in loops and waiting (no external service, no
  refused jobs), about equal in host slips. The largest waste was ordering: the tests were bent
  around twice before they were fixed.

## 7. Key Learnings & Evergreen Upstream

- Check what loads into a session before editing what is written; `@path` in a Claude instruction
  file is an import, and punctuation decides whether it fires.
- Take a baseline before the first edit and pick checks the current text sometimes fails.
- Score from artifacts and the harness's command log; then ask the session what it set aside. No
  `harnez agent` verb for that log was found (1226), so today this needs the provider transcript.
- A rule that is present is not a rule that is followed (1222). Delivery matters more than wording
  on the cheapest tier (1224).
- Tests should assert commands and structure. A test that asserts sentences makes agents bend the
  text to the test. Host or owner: when one blocks a trim, loosen the test first. A commanded agent
  leaves the passage alone and files a ticket, as the skill says.
- A commanded cleanup fits the `orchestrator` role; hand it a ticket, not a long prompt. The skill
  lets the session budget win over reruns, so whoever writes the task should not cap sessions
  below what one retest after the last change needs.
- Run the reviewer on loosened assertions even when the suite is green.

The first four and the agent side of the fifth are in `docs/commands/HarnezCleanup.md`. The host
side of the fifth and the sixth are advice for whoever commands a run and are recorded only here.
`AGENTS.md` covers the last under Development & Review. Nothing further was promoted.

## 8. File & Diff Summary

- **Trimmed sources:** `docs/practices/AgenticLoop.md` and `.lite.md`, `docs/lang/Bash.md` and
  `.lite.md`, `AGENTS.md`, `config.yaml` (rule files, Bash hint, rules header); generated copies
  `docs/AgenticLoop.md`, `docs/Bash.md`, `docs/Search.md`, `.harnez/rules/`.
- **New:** `docs/commands/HarnezCleanup.md`, the sub-study and this story.
- **Skill trimmed by the run:** `docs/commands/HarnezInit.md`.
- **Tests:** `cmd/harnez/agent_test.go`, `internal/claude/rules_init_test.go`,
  `internal/claude/harnez_init_skill_test.go`, golden `cmd/harnez/testdata/init_solo/AGENTS.md`.
- **Outside the repository:** two lines in the owner's global instruction file; global doc copies
  replaced by `harnez apply`.
- **Commits:** `fe021790` to `c176a93d` (PR 164, merged as `b8615f4a`), `c11fa8c2` (owner),
  `bd00d027` to `f6e11a18` (skill, run, tests, study), plus the commit of this story.

## Follow-Up Tickets

- 1222: agents skip `harnez find code`.
- 1223: the generated Git hint contradicts the protected-main Repo Setup.
- 1224: iteration 2: per-agent rule delivery, the renderer comma, remaining docs and skill tests.
- 1226: no `harnez agent` verb that prints a session's executed commands was found; an existing
  flag was not ruled out (filed with this story).
- Non-goal: trimming the cleanup skill to 60 lines now; run it on itself in a later pass.
