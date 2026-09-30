# 650 — Usage Collection Architecture Umbrella

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Architecture
**Related**: [UsageCollection.md](../docs/UsageCollection.md)

---

## Goal

/goal Deliver the staged unified usage, quota, token, and agent-session collection architecture, or stop and report when blocked on a user decision or denied permission.

## Milestones

1. Store read seam — `harnez usage --compact` reads the store API with compatibility import.
2. Collector registry — active provider, meter, and remote-load collection moves behind it.
3. Passive observations — hooks and statuslines ingest normalized source-tagged readings.
4. Session attribution — turn tokens and quota deltas use the shared store API.
5. Consolidation — generated mirrors, parity checks, archival and safe removal of legacy writers.

## Acceptance

- Each milestone has its own ticket, lands in the listed order, and preserves existing export/schema compatibility.
- No reader directly parses a provider cache after its milestone has migrated to the store API.
- Legacy data is archived only after parity, recovery, and rollback checks pass.
