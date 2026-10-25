---
kind: sprint
---

# Two-agent lean sprint 938: split a small study skill from story (2026-10-25)

## Context

Ticket 938: studies are written often, overall stories rarely, but every study loaded the full story skill.
Owner chose the models: developer `d938` (sol:med), reviewer `r938` (opus:med). Searcher skipped: the ticket
named the files; the developer's plan-only first turn checked callers and the write set.

## Outcome

- M1 (split, b0d0b790): new `docs/commands/study.md`; story slimmed; callers lean-sprint, epic, retro, sprint,
  new-project, bug-hunt and the AgenticLoop source point to study; `config.yaml` registration and install test.
- M2 (review fixes, 2428d1de): story's description no longer matches retros; named Ticket Extraction Gate;
  restored worker question, front-matter example and `.harnez/repo.yaml` switch; test checks story refers to study.
- Installed size: study 275 words, story 760 (M1 numbers); a small study loads about 36% of before.
- Verification: host `make test-q1` green after M1; developer `make test-q1` green for M2; host diff review.

## What went wrong

- M1's single test run failed two preset-count tests from a temporary `config.yaml` change; the developer reverted
  it without rerunning. Caught by the quota-1 "changed after the last run" warning; the host ran the owed full suite.
- M1 dropped three small rules and left the story description matching retros. Caught by the opus:med review.
- Host's test job reported "failed" because `grep -c` exits 1 on zero matches; read the log, not the exit code.

## Learnings

- A skill's one-line description decides which skill loads; review it whenever a skill's scope narrows.

## Follow-up tickets

None. The two optional link tweaks (AgenticLoop.md:332, jam.md) are justified non-goals: they never linked story.
