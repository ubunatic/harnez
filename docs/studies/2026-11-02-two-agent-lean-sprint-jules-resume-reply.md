---
kind: sprint
date: 2026-11-02
tickets: [1095]
---
# Lean sprint 1095: Jules resume waits for the real reply

## Setup
- Host Claude Opus 5.5 (decide mode); developer `dev-1095` on `haiku:med` (owner's pick). Searcher skipped: the ticket cited the cause (`track`'s `updateTime` gate) and a read-only developer plan turn confirmed it.

## Flow
- Plan turn confirmed the lines, found the harness trap (no-op `Sleep` with a 60 s deadline spins) and asked two questions. The host answered both and did one GET-only activities read on session 5832791262383075829. It found `originator` agent/user, `progressUpdated` as a reaction, and mixed micro/nanosecond `createTime`. All recorded in the ticket (0eae53ed).
- One implementation turn: M1 fake seams (73db8344), M2 reproduction tests plus fix (85b1df07; three tests red before, green after), M3 jules skill wording (9fe41d43). `make test-q1`: 68 ok, 0 FAIL. Closed 1095 (1d3e315c).

## Host verification
- Removed test lines are fixture updates only (real timestamps and `originator`). Every assertion is kept, and the sort test now uses `02.5Z` vs `02Z`, which exercises parsed-time ordering.
- `go test ./internal/subagent -run Jules` green. `harnez agent status --model jules --id 5832791262383075829` still reads COMPLETED with PR 63.

## Leads
- A session already COMPLETED that receives a send and stays COMPLETED waits out the timeout (accepted, noted by the developer).

## Efficiency
- Two developer turns, no rework. The host's single live read replaced unverified web-search field names.
