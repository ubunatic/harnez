# 1171 — jules skill: skip recently audited tickets and add a batch audit prompt

**Status**: Closed — obsolete: Google stops accepting new Jules work on 2026-11-11, so jules skill improvements will not be used; the batch audit prompt was tried ad hoc in the 2026-11-10 sweep
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Chore
**Related**: 1075, 1123-1135, 1140-1170; study docs/studies/2026-11-06-jules-backlog-cleanup.md

---

## Problem

In the 2026-11-06 single-ticket wave, 5 of 8 delivered audits (1107, 1076, 971, 1102, 959) repeated
a Jules audit from 2026-11-03/04 on unchanged code and added nothing. The skill's "Audit with no
target" picker and the host's manual picks have no check for an earlier audit. Earlier audits sit
under varying headings (`## Audit <date>`, or a prose line "Jules audit (PR GH#69 ...)" in 1102), so
a heading grep misses some.

The backlog batches (1123-1135, 1140-1170) use a multi-ticket prompt that lives only in host scratch
files; the skill has no batch kind.

## Want

- Before starting an audit, skip a ticket whose text contains a Jules audit (`reports/jules/<n>`,
  `Jules audit`, a `## Jules runs` row) newer than the last code change it cites, unless asked.
- Add a `batch` kind with the batch prompt (per-ticket verdict, proposed priority with reason,
  merges, top five) and the wrapper-ticket format of 1131/1140.
- Use the done-when-first prompt from waves 2-4 (owner approved 2026-11-06): a preamble naming the
  role (strict backlog auditor), the approach (list each ticket's `/goal`, `Want`, `Done when` items and
  cite each) and the task; a `close: done` must quote every required item with the code that does it.
  Measured: proposed closes that held rose from 2 of 21 (wave 1) to 4 of 11 (wave 2); cost: more runs
  past 25 minutes and one invented quote, so the host check per reference stays mandatory.
- Ship the host's reference checker with the kind: extract `path:line` plus the quoted line (accept
  `path:line`, `path:file:line`), look for the quote within 10 lines at the audited commit, and score
  N/M; unquoted references count as unscored.
- Close wrapper tickets with `--no-code` once their batch is applied, and record runs in the wrapper's
  `## Jules runs` table.

## Done when

- `docs/commands/jules.md` and the skill source carry both; the root copy is synced.
