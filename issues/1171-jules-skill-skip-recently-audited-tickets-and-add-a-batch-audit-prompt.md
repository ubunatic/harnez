# 1171 — jules skill: skip recently audited tickets and add a batch audit prompt

**Status**: Open
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

## Done when

- `docs/commands/jules.md` and the skill source carry both; the root copy is synced.
