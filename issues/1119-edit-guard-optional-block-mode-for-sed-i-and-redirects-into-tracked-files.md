# 1119 — edit guard: optional block mode for sed -i and redirects into tracked files

**Status**: Backlog — owner: low priority, detection can be flaky and block legit sed/shell use
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 937

---

/goal Decide whether the warn-only edit guard (937) gets an opt-in block mode for Claude host sessions,
and build it only if false positives on legitimate shell use are measured as rare; or stop and report
when blocked on a user decision.

## 1. Problem & Motivation
The owner's rule for Claude is "edit files with Edit/Write, never sed, heredocs or Python". In one
session (2026-11-04, ticket 1118) the host broke it three times: `sed` on `cmd/harnez/main.go`, a
heredoc on ticket 972, and `sed` on the 1118 status line. The 937 guard warned once, after the edit had
already run, so the warning did not prevent it.

## 2. Constraints (owner, 2026-11-04)
- Low priority. Detection of shell edits is heuristic. A block would also stop legitimate uses: sed in
  pipelines, scratch files, generated output, `-update` flows.
- So any block mode must be opt-in (spec/edit_guard.yaml or Local.md), limited to clear cases (`sed -i`
  or `>` into a git-tracked path), and must have an escape hatch.

## 3. Plan (only if pursued)
1. Measure: count the guard's warnings in recent telemetry and sort them into real violations and false
   positives.
2. If false positives are rare, add `mode: warn|block` to spec/edit_guard.yaml (default warn).
