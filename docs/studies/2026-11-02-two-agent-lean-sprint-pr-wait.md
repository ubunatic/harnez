---
kind: sprint
date: 2026-11-02
tickets: [1096]
---
# Lean sprint 1096: `harnez pr wait`

## Setup
- Host Claude Opus 5.5 (decide mode); developer `dev-1096` on `haiku:med` (owner's pick). Searcher skipped: a read-only developer plan turn inspected live code instead.

## Flow
- Plan turn (1m10s) found the existing remote-URL parser in `internal/release/forge.go`, the spec loader pattern and the help golden, and checked PR 78 live (`merged: true` with `state: closed`). Host approved it and recorded decisions in the ticket (ccc97799).
- One implementation turn delivered four milestones: M1 spec + `internal/forge` (3bab4cf2), M2 `internal/prwait` with httptest suite and token-leak test (81526fa3), M3 the command and help golden (a47c3434), M4 `/issue` and sprint skill docs (bb122cac). Developer ran `make test-q1`: 68 ok, 0 FAIL, then closed 1096 (d4bab218).

## Host verification
- Live, installed binary: Codeberg PR 78 exit 0 with merge commit; PR 999999 exit 4 (404); GitHub PR 63 exit 2 (closed unmerged); open PR 76 with `--timeout 2s` exit 3.
- Touched packages rerun green (prwait, forge, release, claude). Release refactor only swaps its private parser for `internal/forge`.

## Deviations and leads
- Only Forgejo hosts listed in `spec/pr.yaml` are detected; another host needs a spec entry.
- A git fetch failure under `--contains` exits 1, not 4.

## Efficiency
- Two developer turns, no rework rounds. The plan turn's live-API check caught the `state: closed` trap before coding.
