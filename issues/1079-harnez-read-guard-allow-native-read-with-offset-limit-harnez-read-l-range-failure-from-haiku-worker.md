# 1079 — harnez read guard: allow native Read with offset/limit; harnez read -L range failure from haiku worker

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: 696

---

## 1. Problem & Motivation
1. The PreToolUse read guard blocks native Read of files over 100 lines and points to `harnez read`. An opus reviewer then fell back to `cat -n`: friction with no token saving.
2. `harnez read` with a line range failed once for a claude haiku:med worker (s3-dev); it was rated via `harnez rate` (details in the rate log).

## 2. Technical Specification / Findings
- A ranged native Read (offset/limit) is already bounded ingestion; blocking it only pushes agents to unguarded shell reads.
- Option: allow Read when offset/limit is set and the range is under the threshold; consider relaxing for the reviewer role.
- Pull the s3-dev failure from the `harnez rate` log to get the exact `-L` argument; may overlap 696 (malformed -L hints).

Reported by the taia session from horg worker runs (epic horg#6), 2026-11-01. Source log: codeberg harnez/horg docs/studies/2026-11-01-agent-performance-log.md, section "Tool Experience".

## 3. Implementation & Verification Plan
- Guard test: ranged Read passes, unbounded Read of a large file still blocks (in enforce mode).
- Reproduce the s3-dev `-L` failure and add a regression test.
