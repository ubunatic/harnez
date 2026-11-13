# 1225 — Add /harnez-cleanup skill and verify it with a terra:high run on one skill

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 1221, 1224

---

## 1. Problem & Motivation

1221 trimmed the core agent docs by hand: measure context cost, shrink, retest on cheap models,
file follow-ups. The owner wants that as a repeatable skill, `/harnez-cleanup <scope>`, written
by the same rules it teaches: assume capable agents, no format prescriptions, only proven
`harnez` commands.

## 2. Technical Specification / Findings

- Source: `docs/commands/HarnezCleanup.md` (76 lines, 3764 bytes), registered in `config.yaml`
  as `harnez-cleanup`, `user-invocable-only`.
- Scope is prose: repository rules by default, named skills, or a full sweep.
- Every `harnez` command in the skill ran in the 1221 session or while writing the skill:
  `harnez agent start --name --role developer --model -f`, `harnez agent resume --name --`,
  `harnez agent models`, `harnez agent delete --force --name`, `harnez issues new -d .`.
- Role choice for a commanded run: `orchestrator` (`spec/agent.yaml`, `spawns: [developer,
  reviewer, advisor]`). It may not edit, so the rewrite goes to one developer helper. No new
  role; a role that both edits and spawns would break the one-writer design.

## 3. Implementation & Verification Plan

- `make test-q1` on the skill and config change.
- Verification run: a `terra:high` orchestrator follows the skill for one existing skill,
  `harnez-init` (`docs/commands/HarnezInit.md`, 59 lines, 3818 bytes, with a test in
  `internal/claude/harnez_init_skill_test.go`).

### Task for the verification run

Follow `docs/commands/HarnezCleanup.md` with scope "the harnez-init skill".

- Budget: at most four test sessions (one cheap model at `low` and `high`, before and after),
  plus one developer helper for the rewrite.
- Test sessions read the skill source by path (`docs/commands/HarnezInit.md`), so no reinstall
  is needed between runs. They work in scratch repositories outside this checkout and never run
  `harnez init` or commit in this repository.
- The developer helper edits `docs/commands/HarnezInit.md` only, on the checked-out branch, and
  commits. No push, no `harnez apply`.
- Skip the study; file no tickets. Write under "Verification run" below: sizes before and
  after, the checks and score per session, what the sessions said they set aside, and every
  place where the cleanup skill was unclear, wrong or missing a step.

### Verification run

The source was 59 lines / 3,818 bytes before; the initial rewrite (`8ca13cf6`) was
28 / 1,612; the final assertion-preserving adjustment (`ef5dc472`) is 45 / 1,769.
The four behavioral runs tested the initial rewrite, **before** that final adjustment.

Independent scoring uses the same five observable checks: G = uninitialized guard and four
setup choices recorded without init; I = initialized scratch init executed; R = regression
assessment recorded; P = local content checked for survival; C = clean scratch commit of init
changes, or N/A when rerun produces none. A pass records evidence of an attempt, not proof of
an exhaustive audit. Artifacts are under `/tmp/cleanup1225-{before,after}-{low,high}`.

- Before low: **5/5** (G I R P C). `transcript.txt` records the guard, init with explicit
  flags, status and diff check, and commit `28bd406`; `REPORT.md` records link and local
  instruction checks. It says no rules were set aside; issue filing was forbidden by the task.
- Before high: **5/5** (G I R P C). Transcript records guard, init, diff check, local AGENTS
  and Makefile preservation and commit `b6939bf`; report identifies tracker links as fenced
  examples. Issue filing and pushing were set aside by task; tracker rules did not apply.
- After low: **5/5** (G I R P C). `TRANSCRIPT.md` records guard, init, diff check and commit
  `5ed259d`; `REPORT.md` records local-section preservation and a link finding. Initial init
  failed on an invalid fixture and succeeded after fixture repair. It says no rules were
  overridden; issue filing, pushing and delegation were excluded by task.
- After high: **4/4, C N/A** (G I R P; no changes to commit). `transcript.log` records guard,
  `harnez init --yes`, clean status, local notes and link checks; `report.txt` records no-op
  rerun. It says no rules were set aside; no issue was filed per task.

All guards were recorded simulations, not interactive confirmations. Fixtures and init flags
varied; after-high's no-op does not test committing a changed init. The after runs called sample
links in `IssueTracking.md` broken, while before-high identified them as fenced examples; this
is an assessment difference, not established evidence of a regression. Reports and transcripts
are self-recorded artifacts, not complete command traces; no injected defect tested fixes or
issue filing. No session ran a consumer test suite. Thus these scores do not establish parity or
prove the final adjustment behaviorally equivalent.

The initial rewrite failed `make test-q1`: `TestHarnezInitSkillInstallToEveryConfiguredTarget`
asserts literal headings and phrases in each installed skill. The final adjustment retained those
assertions without editing tests; `make install`, `git diff --check` and one `make test-q1`
passed. `harnez init --help` was checked; no host init or apply was run.

Cleanup workflow ambiguities and limits observed:

- The ticket's four-session ceiling leaves no room for the skill's repeat-three rule when a
  result decides something; the single observations above are hypotheses, not repeated proof.
- The skill asks for a second vendor when rules reach multiple harnesses, but the ticket budgets
  only low/high before/after on one cheap model; no cross-vendor comparison was performed.
- It calls prose assertions findings rather than reasons to keep prose, but does not say how to
  reconcile a failing required suite with a source-only edit and a request to commit; the final
  adjustment preserves the assertions.
- It does not define a common scoring rubric or how to score no-op reruns and unequal fixtures;
  the rubric and N/A above make those choices explicit.
- It says check commands against help and name only commands run successfully, but does not say
  whether a pre-existing command retained in scope must be executed during the rewrite; help was
  inspected and scratch runs exercised init.
- Retest after each change conflicts with the fixed four-session ceiling after a post-retest
  assertion repair; no behavioral session was added for the final adjustment.
