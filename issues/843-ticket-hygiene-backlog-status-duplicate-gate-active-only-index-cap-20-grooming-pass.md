# 843 — Ticket hygiene: Backlog status, duplicate gate, active-only index, cap 20, grooming pass

**Status**: Open
**Priority**: P1 (High) — every agent session pays the open-list/README context cost
**Severity**: Major
**Category**: Feature
**Related**: 842 (research + decision), 844 (archiving, deferred), 340 (find filters), 697/698 (dup pair)

---

/goal Implement the user's 842 decision (S1 + add-ons) so routine ticket discovery shows ≤20
active tickets, verify it on the live tracker, or stop and report when blocked on a user decision
or denied permission.

## Decision (user, 2026-10-21)

From `docs/studies/2026-10-21-ticket-hygiene-decision.md`:
1. **S1 Backlog status** plus all shared add-ons.
2. Duplicate gate **blocks** `issues new` unless `--force`.
3. Active target **20** (cap value in `spec/`, warn above it).
4. **No archiving / file moves** (links would break) — see 844.

## Milestones

- **M1 — Backlog status + verbs:** lifecycle value `Backlog [— reason]` (value in `spec/`, per
  docs/Spec.md); `is:open` excludes it, `status:backlog` and `status:unresolved` reach it;
  verbs `harnez issues backlog <n…> [reason]` and `promote <n…>`; JSON/lint/completion; normalize
  existing status-line variants (`Done`, `Resolved`, `**Status:**`, …).
- **M2 — Duplicate gate:** OR-ranked `find issues similar "<text>"` over all tickets and statuses
  (title weighted over body, top ~5, status column). `issues new` runs it before reserving and
  exits non-zero on strong matches, printing `issues open|promote <n>` hints; `--force` files
  anyway; `--remote` runs it before `gh issue create`. Validate recall on known pairs (697/698,
  "Closed — duplicate of …" tickets).
- **M3 — Short index + rules:** `issues/README.md` lists active tickets only and links generated
  `issues/BACKLOG.md` and `issues/CLOSED.md`; lint checks completeness across all three. Replace
  `-a status:open` in managed rules (`.harnez/rules/Tools.md`, `Issues.md` sources), IssueTracking
  docs (full + lite, sync root copies), `/issue`, `/roadmap`, `/next` skills. Cap warning in
  `issues lint`/`status` when active > spec cap (20).
- **M4 — Grooming pass (needs user):** dry-run report proposing close (shipped/superseded, e.g.
  Roadmap §9 candidates) and backlog (P3, stale P2, old Drafts) per ticket with reason; user
  reviews and picks the ~20 active; apply in batches with `issues backlog`.

## Verification

Measure before/after on the live tracker: README size (244 KB baseline), `find issues -a
status:open` bytes (41 KB baseline), active count (~249 baseline → ≤20).

## M4 input: proposed active set (2026-10-22, pending user confirmation)

User moved `harnez tui` (776, 778) to the backlog. Active 20: 843, 560 (rescope vs. live
`harnez server` first), 435, 450, 643, 647, 698 (close dup 697), 353, 714, 710, 701, 595, 745,
728, 658, 734, 840, 834, 836, 705. Everything else → Backlog once M1 exists.
