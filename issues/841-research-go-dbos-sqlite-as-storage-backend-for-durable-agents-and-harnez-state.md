# 841 — Research Go DBOS + SQLite as storage backend for durable agents and harnez state

**Status**: Open
**Priority**: P3 (Low) — research feeding Epic 825 (P4); decide before 825 step 2 starts
**Severity**: Minor
**Category**: Research
**Related**: 825, docs/HarnezServerArchitecture.md, docs/studies/2026-10-20-harnez-server-durable-research.md

---

/goal Produce a research report in `docs/studies/` that recommends whether harnez adopts
DBOS Transact for Go (on SQLite) for (a) and (b) below, with evidence, or stop and report
when blocked on a user decision, a denied permission, or missing upstream information.

## 1. Problem & Motivation

Epic 825 plans a "transactional SQLite ledger" for `harnez serve` durable agents (similar
to Pi Durable). DBOS Transact (Go SDK) offers durable workflows, steps, queues and
recovery on a database. The user wants to know whether to use it:

- **a)** as the durability/storage backend for `harnez serve` durable agents, and
- **b)** instead of plain SQLite for any harnez state (sessions, usage, issues index,
  reservations, etc.).

## 2. Questions to Answer

- Does DBOS Go support SQLite (or only Postgres)? Maturity, license, deps, CGO/pure-Go driver.
- Fit with the 825 design: leases, restart reconciliation, idempotent submit, timers,
  cancellation trees, durable approvals. What does DBOS give for free vs. what we build?
- Cost for (b): binary size, startup latency for short CLI calls, schema/migration
  ownership, concurrent access from many harnez processes, inspectability with `sqlite3`.
- Which current harnez stores would move, and which should stay plain SQLite/files.
- Alternatives worth one line each (plain SQLite ledger as in 825, other Go durable libs).

## 3. Approach (multi-agent research)

Per the user's request, fan out parallel read-only researchers, then synthesize:

1. DBOS Go upstream: docs, repo, SQLite support status, API, release cadence.
2. Harnez code survey: current SQLite/file stores and their access patterns.
3. Fit analysis against `docs/HarnezServerArchitecture.md` and the 2026-10-20 study.
4. Optional canary (per `docs/Canary.md`): tiny Go program, DBOS workflow on SQLite,
   kill mid-step, verify recovery; measure startup cost.

Host synthesizes one report with a recommendation per (a) and (b) and records
uncertainties instead of guessing. Follow-up implementation tickets are filed only after
the user decides.
