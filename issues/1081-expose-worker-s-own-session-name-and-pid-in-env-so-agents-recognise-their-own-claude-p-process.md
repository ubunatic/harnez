# 1081 — Expose worker's own session name and pid in env so agents recognise their own claude -p process

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Enhancement
**Related**: 

---

## 1. Problem & Motivation
A haiku:med worker (s6-dev) misreported its own `claude -p` process as a "concurrent agent with the same task". Workers cannot tell themselves apart from peers in `ps` output.

## 2. Technical Specification / Findings
`harnez agent start` could export the session name and the worker's root pid (e.g. `HARNEZ_AGENT_SESSION`, `HARNEZ_AGENT_PID`) and the worker prompt could tell it to exclude these when checking for concurrent agents.

Reported by the taia session from horg worker runs (epic horg#6), 2026-11-01. Source log: codeberg harnez/horg docs/studies/2026-11-01-agent-performance-log.md, section "Tool Experience".

## 3. Implementation & Verification Plan
- Export the vars for every provider; test they reach the child env.
- Add a line to the worker preamble on self-recognition.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-1081 | 2149295139995505177 | none | failed: COMPLETED after 17m34s with no PR and no last message; not re-run |
