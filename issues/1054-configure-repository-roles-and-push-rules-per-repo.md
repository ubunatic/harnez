# 1054 — Configure repository roles and push rules per repo

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `.harnez/repo.yaml`, `AGENTS.md`

---

## 1. Problem & Motivation

Repository instructions can forbid direct pushes even when the repository administrator explicitly authorizes one for a session. Clarify the repository role and applicable push rules in per-repository Harnez settings so agents can distinguish normal team workflow from an explicitly authorized administrator exception.

## 2. Goal & Acceptance Criteria

/goal Provide and document per-repository `.harnez` settings that clearly express repository roles and push rules, including how explicit session authorization interacts with protected branches; verify the resulting agent guidance and behavior, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report explaining why and where to resume.

- Check live configuration, enforcement and recent history before implementation; decide how the new settings relate to existing `git.mode` without duplicating or weakening safeguards.
- Specify safe defaults, precedence and the meaning of administrator authorization; do not imply a setting grants remote permissions or overrides server-side protection. Keep force-push policy explicit.
- Cover configuration validation and representative team/admin workflows with tests and documentation.
