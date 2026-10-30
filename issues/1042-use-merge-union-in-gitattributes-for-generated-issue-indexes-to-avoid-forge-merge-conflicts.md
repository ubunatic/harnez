# 1042 — Use merge=union in .gitattributes for generated issue indexes to avoid forge merge conflicts

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: [[282]], [[847]], [[853]], [[1040]], [[1041]], docs/IssueTracking.md

---

## 1. Problem & Motivation
When concurrent topic branches create or update tickets, each branch modifies `issues/README.md`.
Locally, the custom `merge=harnez-issues-index` driver resolves merges cleanly. However, remote
Git hosting platforms (Forgejo / Codeberg, GitHub) cannot execute clone-local custom merge drivers
from `.git/config` for security reasons.

As a result, remote forges evaluate pull requests with standard 3-way text merge, flagging
spurious merge conflicts in `issues/README.md` and marking PRs as unmergeable even when each PR
touches completely disjoint issue files.

## 2. Technical Specification / Findings
Git provides a built-in merge driver `merge=union`, which is standard across all Git implementations
and recognized by remote forges (libgit2, Forgejo, GitHub).
When `.gitattributes` specifies `merge=union` for generated index files:
- Remote forges resolve concurrent line additions in the table by unioning them instead of generating conflict markers.
- Local merges also use the built-in `union` driver, and the existing `post-merge` hook (`harnez index -d ...`)
  immediately re-sorts and formats the entire table cleanly.
- `internal/claude/init.go` should write `merge=union` for:
  - `issues/README.md`
  - `issues/BACKLOG.md`
  - `issues/CLOSED.md`
  - `docs/README.md`
- `internal/issues/maintenance/rebase.go` should accept both `: union` and legacy `: harnez-issues-index` attributes.

## 3. Implementation & Verification Plan
1. Update `issuesAttributesLines()` in `internal/claude/init.go` to use `merge=union`.
2. Update `.gitattributes` in this repository to `merge=union`.
3. Update `internal/issues/maintenance/rebase.go` validation to accept `union`.
4. Update unit and E2E tests (`cmd/harnez/issues_index_init_merge_e2e_test.go`, `internal/claude/init_test.go`, `cmd/harnez/issues_test.go`, `internal/issues/maintenance/rebase_test.go`).
5. Update `docs/practices/IssueTracking.md` and sync root copy.
6. Verify with `make test-short` / `make test-q1`.
