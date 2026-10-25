---
kind: sprint
---

# Epic 939: host workflow guards (2026-10-25)

## Context

Epic 939 took ticket 937 (edit guard, owner request) and three small open tickets: 916 (`harnez index --stage`),
936 (external-contract test rule) and 915 (close provenance). Baseline: release v0.2.3 (0ec1aaf4). One searcher
(`s939`) checked all four premises up front; each ticket then ran as a lean sprint.

## Outcome

| Ticket | Developer | Reviewer | Commits | Verification |
|---|---|---|---|---|
| 937 edit guard | sol:low | opus:low | 709df88a, ed8dbeaf | host `make test-q1` after M1, developer after M2; live hook probe 4/4 |
| 916 index --stage | luna:high | host diff | ac726732 | developer `make test-q1`, scratch-repo staging check, host no-op run |
| 936 contract-test rule | haiku:high | host diff | 68618f27 | root-copy test; diff is 4 doc lines |
| 915 close provenance | luna:high | host diff | e4ffd8f5 | developer `make test-q1`; host closed 915 with `--model/--session` live |

16 commits since v0.2.3; outside `issues/`, 28 files, +969/-72 (mostly tests and the help golden).

## What worked

- Plan-first turns: every developer plan named the right files, so each got write access after one host look.
- Second-tier review on the risky ticket only: opus:low found two false negatives and three false positives in
  937 M1 (absolute repo paths, heredoc bodies, `&&` chains), all fixed in M2. The three small tickets needed no
  reviewer beyond the host diff.
- haiku:high handled a pure doc ticket in one turn with a clean diff.

## Post-mortem

- 937 M1: the developer's test run stopped at `go vet`; the host's full run was the first green one. The
  review then showed green tests did not cover the hook's real inputs.
- 916 has a known limit: an index file rewritten by an earlier run and left unstaged is not staged.
- `Closed-By` is removed when a re-close passes neither flag nor env; a reopened ticket keeps its old line until
  the next close. Both are acceptable for now; no ticket.
- Host slip risk: auto mode invited shell edits; the host kept to Edit/Write, and the new 937 guard now warns
  when that rule is broken.

## Learnings

- The external-contract rule (936) came from this epic's own evidence and now lives in AgenticLoop §2 and the
  lean-sprint handoff checklist.
- Reviewer budget follows risk: one opus:low review on a hook that runs on every Bash call; host diff for docs
  and small CLI flags.

## Follow-ups

None filed; the limits above are recorded as accepted.
