# 1075 — Use one-shot Jules runs for report-only audits, with a jules-audit skill

**Status**: Closed — Delivered: 25 one-shot Jules runs, reports in reports/jules/, 80 of 86 checked references held up, and the /jules-audit skill. Outcome section records what is worth keeping.
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

**Result: passed.** All 5 runs completed unattended in 10 to 16 minutes and each added only its report (GitHub PRs 38 to 42). All 11 `path:line` references I checked hold up at `c7429fb7`.

Weaknesses, which the prompt for later batches addresses:

- 950 gave no `path:line` reference, and 1031 and 966 only one each.
- 1031 searched only for lowercase `autocompact`, so it missed the `auto_compact` settings in `internal/subagent/agentspec.go`.
- 923 cites line 53 of a file while listing a command that read only its first 30 lines, so listed commands do not prove the evidence.
- 950 names `docs/GoRelease.md` and `docs/Go.md`, which are generated copies. The sources are in `docs/practices/` and `docs/lang/`.

## Steps after a passing pilot

1. Run three more audit batches of 5 tickets each.
2. Create a `/jules-audit` skill from the prompt and checks that worked.
3. Try three other one-shot uses: a code audit of one package, a check of docs against code, and independent drafts of a failing test for one hard ticket.
4. Wrap up: collect the reports on Codeberg, close the PRs and delete their branches on GitHub, and record what is worth keeping.

## Notes

- `git push github origin/main:main` before each batch, because Jules reads GitHub.
- `harnez agent start` handles one session and waits on it. A batch command that starts up to the concurrency limit and collects reports is only worth building if these runs prove useful.
- Not confirmed: whether API runs count against the 100 a day, and the limit of 15 runs at once for this plan.

## Outcome (2026-11-01)

All steps are done. Jules ran 25 times: 20 ticket audits and 5 trials of other uses. The per-report results are in `reports/jules/README.md`.

- **Scope:** all 23 PRs changed only the file they were allowed to. The other 2 runs stopped to ask a question and opened no PR, one of them despite a prompt line forbidding that. Their findings were recovered from their last messages.
- **Accuracy:** 80 of 86 checked `path:line` references held up, which is 93%. Only the 927 report fell below 80%, after a prompt line of mine invited placeholders.
- **Speed:** 3 to 40 minutes a run. Ten at once all ran, so the limit of 15 is still unconfirmed.
- **Skill:** `/jules-audit` holds the prompt and checks that worked.

Worth keeping, best first:

1. **Independent drafts of a failing test.** Three runs on 1017 cost almost no review, because running a test checks it. They exposed the real defect: `issues rebase` cannot repair tickets numbered 1000 or above. Neither the ticket nor the code audit had found that.
2. **Ticket audits.** They are useful for finding what is stale. Two of 20 tickets were already done (951, 1053) and 2 had an out-of-date claim (923, 971). Most verdicts only say a ticket is still valid, so the value is in the `path:line` starting points. Jules's reasoning about what to change next is the weakest part.
3. **Docs check.** It found 5 real mismatches in `docs/CLIDesign.md`. Applied 2026-11-11 after its late PR GH#165 (closed) gave the line references.
4. **Code audit.** It gave one minor finding and missed the defect above, so it is the least useful.

Applied from the reports: closed 951 and 1053; corrected 923 and 971; rewrote the cause in 1017 and raised it to P2. Everything else stays a lead in `reports/jules/`.

GitHub PRs 38 to 60 are closed and their branches deleted. The one other GitHub branch, `fix-issues-rebase-6379860427770255774`, belongs to the earlier PR 37 of ticket 1017, so it stays. Whether API runs count against the daily 100 remains unknown; 25 were used today without a limit showing up.
