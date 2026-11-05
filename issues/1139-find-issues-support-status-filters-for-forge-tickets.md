# 1139 — find issues: support status filters for forge tickets

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1116, 1138; taia (codeberg.org/harnez/taia)

---

## Problem

Seen 2026-11-06 in `../taia`, whose tickets live on Codeberg:

```
$ harnez find -d ../taia issues status:open
Error: find issues: filter "status:open" is not supported for forge tickets; use plain search terms
```

`harnez find -d <repo> issues status:open` is the documented way to list active tickets (global
instructions, Issue Tracker Discovery), so agents working on a forge-tracked repo hit this first.
The forge API already filters by state (Forgejo `state=open|closed|all`).

## Want

- `status:open` and `status:closed` map to the forge's issue state; other `status:` values give a
  clear error that lists the supported ones.

## Done when

- `harnez find -d ../taia issues status:open` lists open taia issues; a test with a fake forge
  checks the `state` query parameter.
