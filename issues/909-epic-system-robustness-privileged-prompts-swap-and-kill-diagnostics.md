# 909 — Epic: system robustness, privileged prompts, swap and kill diagnostics

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 865, 885, 887

---

/goal Complete the tickets below with `/lean-sprint` runs; each closes only when its own acceptance holds, a
shortfall gets its own ticket. Stop and report when a worker loops, a blocker cannot be solved, or a ticket needs
an owner decision (885 changes system swap: confirm with the owner before any live change). Circuit breaker: on no
progress, clear the goal and write a failure report here.

## Tasks

| # | Ticket | Pri | What | Status |
|---|---|---|---|---|
| 1 | 887 | P2 | Signal-kill diagnostics M2: OOM/cgroup checks, stderr at command boundary | M1 done, M2 open |
| 2 | 865 | P2 | Privileged subprocesses, PAM biometric prompts, stderr visibility | Canary exists; host run and rules owed |
| 3 | 885 | P2 | Optional disk swap backstop for zram-only systems so tmpfs /tmp can spill | Open |

Sprint groups and worker tiers are set by `/epic 909` refinement.
