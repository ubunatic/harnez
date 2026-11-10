# 1093 — Failed Jules start leaves a session record that blocks the retry name

**Status**: Closed — obsolete: Google stops accepting new Jules work on 2026-11-11 and shuts Jules down on 2026-12-31 (see the jules skill); this fixes a Jules start/driver path that will not run again
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1088, 1092

---

## 1. Problem & Motivation

Reported from loom on 2026-11-01. `harnez agent start --model jules --name jules-audit-368` failed with "no github.com remote in /home/alfiz/projects/loom". The retry, once the remote was added, failed with `session name "jules-audit-368" is already in use` and needed `harnez agent delete --force`.

## 2. Technical Specification / Findings

The Jules preflight (finding the GitHub remote, `githubRemote` in `internal/subagent/jules.go:303`) runs after the session record is created. Not verified: the exact place where start writes the record.

## 3. Implementation & Verification Plan

Run the Jules preflight (GitHub remote, `github_projects`, key) before writing a session record, or remove the record when start fails before Jules gets a session ID. Test: a start with no github remote leaves no record, and a retry under the same name succeeds.
