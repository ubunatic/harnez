# 1273 — Add more `harnez pr` verbs to replace `fj` for pull requests

**Status**: Closed — added list/show/comment/create/edit/close/reopen/merge verbs to harnez pr; make test-q1 passed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `cmd/harnez/pr.go`, `internal/forge/`, `issues/1247-allow-forge-issues-edit-via-codeberg-token.md`, `issues/1261-pr-wait-keeps-polling-a-pr-with-merge-conflicts-exit-so-the-waiter-can-react.md`

---

## Goal

`/goal Expand harnez pr with the core pull-request verbs needed for routine Codeberg/Forgejo and GitHub workflows (listing, inspecting, commenting, merging, closing, and creating/editing PRs) using harnez's native forge API and token resolution so agents and users no longer need to rely on fj, or stop and report when blocked on a user decision, missing forge credentials, or denied permission. On a loop, hard architectural blocker, or no progress, clear the goal and write a failure report explaining why it failed.`

## 1. Problem & Motivation

`harnez pr` currently exposes only `wait`. Routine pull-request operations (listing open PRs, viewing a PR's metadata/diff/comments, commenting, merging, closing, or creating/editing PRs) still require falling back to `fj` (or `tea` / `gh`). `fj` has recurring friction with auth, wrappers, host flags, and token reuse, whereas `harnez` already has native Forgejo/GitHub HTTP clients, host/repo detection, and token resolution (`internal/forge/`, including `token_env` fallback and `harnez forge status`).

## 2. Scope & Open Questions

Before starting implementation, check live `cmd/harnez/pr*.go`, `internal/forge/`, and recent commits to align with the current forge client and CLI patterns.

Candidate verbs to consider (confirm exact set and flags during implementation):
- Read verbs: `list`, `show` / `view` (metadata, status, checks, comments, optional `--json`)
- Write verbs: `comment`, `create` / `new`, `edit`, `merge`, `close`, `reopen`
- Consistent `--host` / `--repo`, `--dry-run` for writes, host-only write guard matching `harnez issues`, and native Forgejo/GitHub API support without shells out to `fj`.

## 3. Acceptance Criteria

- Core read and write `harnez pr` verbs work natively against Codeberg (Forgejo) and GitHub using harnez's forge token resolution (`harnez forge status` / `internal/forge`), without requiring `fj`.
- Write verbs support `--dry-run` and follow harnez's host-only write safety rules and error diagnostics.
- Unit tests cover API requests, auth/token selection, and CLI flag/output behavior; `make test-q1` passes and help golden is updated.
