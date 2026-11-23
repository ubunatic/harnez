# 1272 — issues new warns that open PRs were not checked (HTTP 404) on every call when origin is not a reachable forge repo

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1138

---

## 1. Problem & Motivation

Reported by the splitty host session (taia, 2026-11-23): every `harnez issues new` in
`/home/alfiz/projects/splitty` warns
`open PRs not checked on origin (forge API request failed: HTTP 404 API request to codeberg.org)`.
The warning repeats on every call and says nothing about the cause, so agents learn to ignore it.

## 2. Technical Specification / Findings

Probed 2026-11-23 (anonymous API):

- `ubunatic/splitty` on Codeberg is public (`"private":false`, `"has_pull_requests":true`) but
  **empty** (`"empty":true`, nothing pushed). `GET /repos/ubunatic/splitty` and its `/issues`
  return 200, while `GET /repos/ubunatic/splitty/pulls` returns 404. `ubunatic/harnez/pulls`
  returns 200.
- The warning comes from the open-PR claim scan, `internal/issues/claims.go:79`.

## 3. Implementation & Verification Plan

- A 404 from the pull list of a repo that exists and is `empty` means there are no open PRs:
  treat it as an empty list, or say "remote repo is empty" instead of a bare HTTP 404.
- Keep the warning for a real failure (private repo without a token, unreachable host).
- Test: stub forge with an empty repo (repo 200 with `empty:true`, pulls 404) prints no
  warning; repo 404 still warns.
