# 1054 — Configure repository roles and push rules per repo

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `.harnez/repo.yaml`, `AGENTS.md`

---

## 1. Problem & Motivation

Repository instructions can forbid direct pushes even when the repository administrator authorizes them. The owner specifies that, from this machine, issue filing, epic planning and documentation-only changes may be committed and pushed directly to `main`; feature work generally goes through a pull request. Represent this distinction in per-repository Harnez settings and guidance, rather than requiring a PR for every tracker or documentation change.

## 2. Goal & Acceptance Criteria

/goal Provide and document per-repository `.harnez` settings that clearly express repository roles and push rules, including how explicit session authorization interacts with protected branches; verify the resulting agent guidance and behavior, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report explaining why and where to resume.

- Check live configuration, enforcement and recent history before implementation; decide how the new settings relate to existing `git.mode` without duplicating or weakening safeguards.
- Specify safe defaults, precedence and the meaning of administrator authorization; do not imply a setting grants remote permissions or overrides server-side protection. Keep force-push policy explicit.
- Cover configuration validation and representative team/admin workflows with tests and documentation. Distinguish eligible documentation-only changes from feature or mixed changes, and identify the authorized machine/role without treating self-assertion as remote permission.
