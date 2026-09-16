# 375 — Append quota window snapshots to quota-history.jsonl on cache refresh

**Status**: Open  
**Priority**: P2 (Medium)  
**Severity**: Minor  
**Category**: Feature  

**Related**: [023](023-usage-command-token-quota-tracking.md), [033](033-usage-shared-quota-cache.md), [docs/studies/2026-09-16-agent-token-and-quota-trackability-status.md](../docs/studies/2026-09-16-agent-token-and-quota-trackability-status.md)  

---

## 1. Context & Motivation

Currently, `harnez usage` fetches live rate limits and capacity windows from upstream AI providers (Anthropic, Antigravity/Gemini, OpenAI Codex) and writes them to local disk caches (`harnez-quota-cache.json`).

However, `harnez-quota-cache.json` only stores the single **latest** snapshot. When a rolling quota window resets (e.g. at the end of a 5-hour session window or 7-day weekly period), previous capacity records and burn-down trajectories are overwritten and lost.

Because upstream providers return percentages rather than absolute token denominators (e.g., "83% remaining" without exposing total token capacity), tracking historical quota snapshots over time is the only reliable way to reconstruct:
1. **Capacity Burn-Down Velocity**: Rate of quota drain across heavy sprint sessions.
2. **Weekly Usage Trajectories**: Long-term utilization patterns against subscription limits.
3. **Reset Cycle History**: Identifying exact historical reset boundaries and period rollovers.

---

## 2. Proposed Architecture & Design

1. **Snapshot Sink**:
   * Append-only JSONL file located in the user's harnez / agent data directory:
     * Unified location: `~/.claude/harnez/usage-history/quota-history.jsonl` (or agent-specific data dir).
2. **Schema**:
   ```json
   {
     "timestamp": "2026-09-16T20:37:14Z",
     "agent": "agy",
     "group": "Gemini Models",
     "window": "Weekly Limit Remaining",
     "used_percent": 17,
     "remaining_percent": 83,
     "reset_at": "2026-09-23T09:22:53Z",
     "duration_left_ms": 571538071
   }
   ```
3. **Deduplication & Throttling**:
   * Only append a new snapshot row when:
     - The window percentage or `reset_at` changes, OR
     - At least 15–30 minutes have elapsed since the last recorded snapshot for that window.
   * This prevents unbounded log growth during rapid `--watch` intervals.
4. **Integration Points**:
   * Hook directly into `internal/usage/` whenever `WriteQuotaCache()` succeeds.

---

## 3. Acceptance Criteria

- [ ] `internal/usage/` appends quota window states to `quota-history.jsonl` upon fresh cache updates.
- [ ] Record deduplication ensures identical percentage readings within a short window do not spam the log file.
- [ ] Snapshots capture agent ID, model group, window name, used/remaining percentages, and `reset_at`.
- [ ] Unit tests in `internal/usage/` verify snapshot format, append behavior, and throttling.
- [ ] `harnez index` verifies documentation and tracker consistency.
