# 842 — Research ticket hygiene at ~900 tickets: archive/backlog tiers, short open lists, duplicate detection

**Status**: Closed — User decided: S1 Backlog status + add-ons, blocking dup gate, cap 20, no archiving yet; implementation in 843, archiving parked in 844
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Research
**Related**: 234 (roadmap skill), docs/IssueTracking.md

---

/goal Deliver three independent research reports plus one aggregated decision matrix that let
the user choose a ticket-hygiene scheme for harnez's file-based tracker, or stop and report when
blocked on a user decision, a denied permission, or an unavailable model.

## 1. Problem & Motivation

Harnez has 842 tickets. On 2026-10-21, `harnez find issues -a status:open` returned ~247 active
tickets (223 plain `Open`, plus Blocked/In Progress/annotated Open). Concerns from the user:

1. **Context cost:** agents list open tickets often, and that list keeps growing.
2. **Hygiene:** there should be far fewer active tickets (target ≤100, ideally ~20). Old tickets
   should move to a backlog/archive that is read only on explicit request, not in the normal flow.
3. **Duplicates:** a newly filed ticket must still be matched against old archived tickets; a
   match should reopen or promote the archived ticket instead of creating a duplicate.

Constraint: stay with the file-based `issues/*.md` system and `harnez issues`/`harnez find`.

## 2. Research Setup (user-specified)

Three competing researchers, each read-only except for its own report:

| Researcher | Model | Report |
|---|---|---|
| R1 | `sol:med` | `docs/studies/2026-10-21-ticket-hygiene-sol.md` |
| R2 | `opus:med` | `docs/studies/2026-10-21-ticket-hygiene-opus.md` |
| R3 | `argon:low` | `docs/studies/2026-10-21-ticket-hygiene-argon.md` |

Each report proposes **3 promising solutions** with pros and cons, covering: where archived
tickets live (status field, directory, separate index), how listing defaults change, how
duplicate search reaches archived tickets and what happens on a match, migration of the
existing ~247 active tickets (grooming criteria), and impact on `harnez issues`/`find`/index,
docs/IssueTracking.md and other harnez-managed projects. Record uncertainties; no code.

## 3. Aggregation (host)

Review all 9 proposals, merge similar ones, and write one decision matrix (rows = distinct
solutions, columns = context cost, hygiene effect, duplicate detection, migration effort,
tooling change, risks) with a short pros/cons summary and a recommendation, in
`docs/studies/2026-10-21-ticket-hygiene-decision.md`. The user then decides; implementation
tickets are filed only after that decision.

## 4. Research delivered (2026-10-21)

Reports: sol, opus, argon (`docs/studies/2026-10-21-ticket-hygiene-*.md`); matrix:
`docs/studies/2026-10-21-ticket-hygiene-decision.md`. 9 proposals → 4 solutions (Backlog status,
tier field, directories, NOW list). Recommendation: Backlog status + duplicate gate + README split
+ rule fix + cap lint. Host verified sizes: `issues/README.md` 244 KB, `-a status:open` 41 KB.
**Awaiting user decision** (4 questions in the matrix doc).
