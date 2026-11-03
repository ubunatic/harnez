# 1116 — Forge-tracker mode: harnez issues show/find for repos whose tickets live on Codeberg or GitHub

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1115

---

/goal Let `harnez issues show N` and `harnez find issues` read tickets from the repo's forge when
`.harnez/repo.yaml` selects a forge tracker, verified against harnez/taia on Codeberg; or stop and
report when blocked on a design decision or missing forge credentials.

## 1. Problem & Motivation
Requested by the taia session (2026-11-03). taia keeps its tickets on Codeberg (`harnez/taia`) and
has no `issues/` dir. Both commands failed with `scan issue files: stat .: no such file or
directory`. Stopgap shipped: `Scan` now says there is no local tracker and points to the forge.

## 2. Findings
- `issues.tracker` enum is `harnez | none` (`spec/schemas/repo.schema.json`); a forge value
  (e.g. `forge`) is missing. Forge endpoints belong in `spec/`, not Go defaults.
- `harnez pr wait` already talks to the Codeberg and GitHub APIs; reuse its client.
- Scope decided by taia (2026-11-03): read-only show/find only; `fj` already covers create/close.
  Stays P2, behind harnez P1 work. Open: auth for private repos.

## 3. Plan
1. Add the tracker value and spec entries; read-only show/find via the forge API.
2. Verify in `~/projects/taia`: show an existing ticket, find by text, clear error offline.
