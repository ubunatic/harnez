# 1075 — Use one-shot Jules runs for report-only audits, with a jules-audit skill

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1071, 1073, 1074

---

/goal If the pilot passes, finish three more audit batches, a `/jules-audit` skill and trials of three other one-shot uses, with every report collected on Codeberg and the outcome recorded here, or stop and report when the pilot misses its bar or when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Why

The owner has 100 free Jules runs a day. Google stops accepting new work on 2026-11-11, so at most about 1,000 runs are left (from 2026-11-01). Nobody can review 100 code PRs a day, but a report can be checked quickly, and the tracker holds 102 active tickets against a cap of 30.

## Rules

- One shot: one prompt per run, no follow-up messages. For now, never ask Jules to fix anything.
- Use the content: keep a useful report, discard a bad one, and do any finishing work ourselves.
- A report is a lead, not a fact: check its evidence against the code before changing a ticket.

## Pilot (started 2026-11-01)

Audits of tickets 923, 950, 966, 985 and 1031, from GitHub `main` at `c7429fb7`, with sessions `jules-audit-<number>`. Each run may add only `reports/jules/<number>.md`.

Passes when at least 4 of the 5 reports add only that file and at least 80% of the checked `path:line` references hold up.

## Steps after a passing pilot

1. Run three more audit batches of 5 tickets each.
2. Create a `/jules-audit` skill from the prompt and checks that worked.
3. Try three other one-shot uses: a code audit of one package, a check of docs against code, and independent drafts of a failing test for one hard ticket.
4. Wrap up: collect the reports on Codeberg, close the PRs and delete their branches on GitHub, and record what is worth keeping.

## Notes

- `git push github origin/main:main` before each batch, because Jules reads GitHub.
- `harnez agent start` handles one session and waits on it. A batch command that starts up to the concurrency limit and collects reports is only worth building if these runs prove useful.
- Not confirmed: whether API runs count against the 100 a day, and the limit of 15 runs at once for this plan.
