# 825 — Epic: harnez serve, durable any-client agent server

**Status**: Open
**Priority**: P4 (Backlog) — deferred 2026-10-20: finish other open work first
**Category**: Feature
**Related**: 776, 476, 587, 611, 293, 463, 646

## Goal

Build `harnez serve` as designed in [docs/HarnezServerArchitecture.md](../docs/HarnezServerArchitecture.md):
any client to one server that runs native CLI sessions durably, with optional bounded
unattended goals. Research: [docs/studies/2026-10-20-harnez-server-durable-research.md](../docs/studies/2026-10-20-harnez-server-durable-research.md).

## Steps (file as child tickets when started)

1. Extract start/resume/compact orchestration into a service; per-provider capability matrix.
2. Transactional SQLite ledger, idempotent submit, import existing sessions.
3. `harnez serve` supervisor with leases, restart reconciliation, timers; kill-test canary.
4. Task handles, live view vs. durable cursor/outbox, move CLI/MCP/TUI onto it.
5. Bounded unattended goals, durable approvals, cancellation trees, "Running now" view.
6. ACP/web/chat/cloud adapters, cross-provider handoffs, optional Pi Durable worker (API models only).

Open question: confirm Anthropic's current policy on Claude subscription tokens in third-party harnesses.
