# 1202 — harnez issues has no way to edit a forge issue body; agents fall back to tea

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1196, 1197, 1201, 1067

---

/goal Agents on a forge-tracker checkout can update a forge issue's body (and title) through `harnez issues`, so they stop shelling out to `tea`/`fj`; or stop and report when the forge API or auth blocks it.

## 1. Problem & Motivation
Feedback from another session (2026-11-09): "I updated the issue with tea because harnez issues can't edit an issue body."
On a forge tracker, `harnez issues show` is read-only and no verb edits a body. Agents bypass harnez, so the edit skips its token handling and telemetry.

## 2. Findings
- `harnez issues --help`: `show` on a forge origin is "read-only"; no `edit` verb exists.
- Forgejo/Gitea: `PATCH /repos/{owner}/{repo}/issues/{index}` takes `title` and `body` (unverified in harnez; canary first, docs/Canary.md).

## 3. Plan
- `harnez issues edit <n> [--title T] [--body-file F|-]` on forge trackers, through `internal/forge` (same token path as `GetJSON`).
- Local trackers: refuse with a hint to edit the file, or leave out of scope.
- Tests: fake forge server checks method, path and payload; no token in errors.
