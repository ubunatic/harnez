# 1088 — Add a /jules skill for one-shot implement, experiment and ideas runs

**Status**: In Progress — skill build on issue/1088-jules-skill
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1071, 1075

---

/goal Ship a `/jules` skill that covers one-shot audit, implement, experiment and ideas runs, and try each new kind on a real task before Google stops accepting Jules work on 2026-11-11. Stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Why

The owner wants one-shot Jules runs for real work as well as ticket reviews: Jules may code, try something and report back, or suggest better code. `/jules-audit` (`docs/commands/jules-audit.md`) cannot do that, because its rules and prompt allow only one report file and forbid fixes. Ticket 1075 kept that limit "for now" while it tested the approach. Its 25 runs had clean scope, and 80 of 86 checked references held up.

## Scope

One skill with these run kinds, each one prompt and one PR, with no follow-up messages:

| Kind | What Jules does | Result |
| --- | --- | --- |
| Audit | Today's `/jules-audit` | Report |
| Implement | Codes a well-specified ticket and runs its tests | Code PR and report |
| Experiment | Tries an approach and may change code freely | Report, with the code only as evidence |
| Ideas | Reads one area and proposes improvements | Report with proposals that cite the code |

Decide in this ticket whether `/jules-audit` stays as an alias or is replaced, and update its callers in the same change.

## Rules to carry over or add

- Keep the batch steps and evidence checks from `/jules-audit`: GitHub sync, no push mirror or pruning, and a report is a lead, not a fact.
- Code PRs get a normal review before merging. Jules is never asked to rework one; the host finishes any fixes through the normal workflow.
- Check an experiment's or implementation's claims by running its tests, not by reading the report.
- Include independent drafts of a failing test: 1075 ranked them as the most useful use so far.
- Starting a run in a repository needs that repository in `github_projects` and connected at jules.google.com. Say so in the skill, because both errors stopped a uman audit on 2026-11-01.

## Decision

- `/jules` replaces `/jules-audit`, with no alias: Jules takes no new work after 2026-11-11, so an alias would outlive nothing. Reason: one skill to maintain.
- Lean sprint without a Searcher: the host already read the skill and all its callers (`config.yaml`, `internal/claude/claudeskills_test.go`).
- M1 is the skill, pushed to `main`. M2 is one real run of each new kind, started from `main`, because a start on another branch skips the GitHub sync.

## Evidence from uman (peer report, 2026-11-01, four runs at `0506459`)

- Implement runs took 24 and about 39 minutes and gave usable code; report-only runs took 7 and 8 minutes.
- Ideas: 4 of 5 proposals restated `CLAUDE.md`, and the fifth understated a real bug. Prompts should exclude known gaps, and a report's severity needs checking.
- Experiment: the run skipped an approach the prompt asked for, so the prompt needs a checklist the report must answer.
- A start that hits its wait limit exits 0, so exit 0 does not mean the run finished.

Harnez bugs from the same report are in 1089.

## Skill changes owed before closing

- Implement prompt: pick the smallest option yourself where the ticket leaves a choice. The first run on 1087 stopped to ask.
- Ideas prompt: exclude gaps already listed in `CLAUDE.md` and open tickets.
- Experiment prompt: a checklist of requested approaches that the report must answer.
- Section 3: exit 0 can mean the wait ran out, so read the state in the output.
- Findings capture (owner, 2026-11-01): once a run has finished and its PR is read, every finding that survived the evidence check goes into a new issue in that repository's default tracker, through `/issue`. The exception is a quick fix, which is made directly as a small code change through the normal workflow.

## Done when

The skill is installed by `harnez init`, and one real run of each new kind is recorded here with what was kept.
