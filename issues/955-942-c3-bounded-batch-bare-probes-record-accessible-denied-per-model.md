# 955 — 942 C3: bounded batch bare probes record accessible/denied per model

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 942 (epic, ## Decision 5 and bare-probe rule), 946 (C1 probe), 954 (C2 access.yaml)

---

/goal `harnez models discover --probe` runs 946's bare probe for each catalog model after one confirmation,
within the spec budget (max calls, token cap, timeout), stops on the first quota or auth error, and records
`accessible` / `denied` / `unknown` observations in access.yaml (954's writer); ambiguous errors and timeouts
stay unknown. Tests cover budget cut-off, stop on quota error, and a partial run that removes nothing. Or stop
and report on a design choice. Circuit breaker: on a loop or no progress, clear the goal and write a failure
report here.
