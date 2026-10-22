# 851 — bug-hunt skill: pick P1/P2 bugs, re-verify, close obsolete ones, fix the rest the way forward

**Status**: Open
**Priority**: P2 (Medium) — turns a working session pattern into a repeatable workflow
**Severity**: Minor
**Category**: Workflow
**Related**: docs/commands/lean-sprint.md, 843 (active-ticket cap), 849/728/850/848/701/658/734/595/745/714/710/450 (first hunt)

---

/goal A `/bug-hunt` skill (source `docs/commands/`, mapped in `config.yaml`, installed by `harnez apply`) runs the
workflow below on the active P1/P2 bugs without user check-ins, and stops with a report when it loops or is
blocked; verified by one real run on this repo, or stop and report when blocked on a user decision.

## Workflow (as run on 2026-10-22)

1. **Pick**: active tickets (`harnez find issues -a status:active`) with Category Bug (or bug-like, e.g.
   Reliability) and P1/P2. Order: freshest and blockers of other work first; pair tickets sharing code.
2. **Re-verify, read-only**: cheap searchers check each ticket against live code and recent history:
   STILL-CURRENT / PARTLY / OBSOLETE with evidence, root cause file:line, way-forward fix, write set, test.
   Old tickets may describe a design that has moved on; a misleading title is noted (734).
3. **Record** the dated live-check section in each ticket and commit it (the ticket stays the only medium).
4. **Decide per ticket**: obsolete → close with evidence; not reproducible → make the next occurrence
   diagnosable, then Block "waiting for a live repro" (714/710); current → fix the way forward, not as first
   written; out-of-scope parts named in the close reason (450 Loom hosting).
5. **Fix**: one developer at a time (shared repo), batches of related tickets, one commit per ticket,
   one `make test-q1` per batch, close only when acceptance is met.
6. **Review**: host checks diffs and runs the live check itself (real `harnez apply`, live probes, real-flow
   git e2e, clean `git status` after `harnez init`); reopens with M2 pre-work when acceptance is missed (850:
   developer closed with 119 still above the cutoff).
7. **Teardown**: rate and delete each agent; study note in `docs/studies/`.

## Open questions

- Own skill or a `/lean-sprint bugs` mode? The steps reuse lean-sprint roles; the differences are the
  selection, the read-only re-verify pass over many tickets, and the obsolete/blocked outcomes.

## Decided

- Parallel research is allowed: read-only searchers may run in parallel with each other and with the one active
  developer (worked on 2026-10-22). Only writers (developer, host ticket commits) are serialized; the host commits
  only the files it staged.
