# 609 — Keep finder query text in local telemetry (opt-in)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 515 (telemetry home), neus 022 (search test set)

---

## 1. Problem & Motivation
Peer request from neus (user approved, 2026-09-27). neus 022 builds its search test set from real
`harnez find` queries, but harnez telemetry (`tool_calls`) strips the query arguments. Only 2 real queries
could be recovered from transcripts; 110 had to be hand-written.

## 2. Technical Specification / Findings
- Wanted per finder call: query string, `--root`/`--kind`, and the hit paths the finder returned
  (at minimum the query).
- Must be opt-in and local-only (never leaves the machine). Privacy switch and retention limit are ours to choose.
- Store location follows `docs/Telemetry.md` (515).

## 3. Implementation & Verification Plan
- Config/env switch, default off; when on, record query, root, kind and hit paths in the local telemetry DB.
- Retention limit (e.g. age or row cap) enforced on write.
- Export for sampling (e.g. `harnez stats` subcommand or documented SQL) so neus can label real queries.
- Tests: off by default records nothing; on records fields; retention prunes.
