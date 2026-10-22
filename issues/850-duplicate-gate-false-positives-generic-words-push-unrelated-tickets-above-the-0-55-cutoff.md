# 850 — Duplicate gate false positives: generic words push unrelated tickets above the 0.55 cutoff

**Status**: Open
**Priority**: P2 (Medium) — the gate blocks `issues new` on strong matches
**Severity**: Minor
**Category**: Bug
**Related**: 843 M2 (duplicate gate, 029d9a6b), 849

---

/goal `harnez find issues similar` stops scoring unrelated tickets as strong matches because of shared
generic words, while it still catches the known true matches; verified on the live tracker, or stop and
report when blocked.

## Observed (2026-10-22)

`harnez find issues similar "harnez apply does not update an existing installed skill SKILL.md"` scored
three unrelated closed tickets at 0.583, above the 0.55 cutoff: 119 (telemetry hook into apply), 355
(apply/init don't prune sections), 716 (skills into Pi). The shared words were generic (`harnez`,
`apply`, `install`, `skill`). The title as finally filed (849) passed, so the effect depends on wording.

## Ideas

- Weight terms by rarity across the tracker (IDF), so `harnez`/`apply` count little.
- Stopword list for project-generic terms in `spec/issues.yaml`.
- Raise the cutoff only after measuring.

## Check against

True matches from 843 M2: the usage-error text → 714 (0.571), the lean-sprint paths text → 845 (0.733),
697/698. The fix must keep these, drop the three above, and keep an unrelated title (≤0.40) low.
