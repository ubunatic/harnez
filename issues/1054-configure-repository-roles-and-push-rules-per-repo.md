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

/goal Provide per-repository role and push settings so an authorized admin can routinely push new issues and automated index updates directly to `main`, while non-admin contributors can submit ongoing issue and documentation updates through their own reusable AGit pull request targeting `main`; feature and code changes use separate topic branches and pull requests. Verify guidance and behavior, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report explaining why and where to resume.

- Check live configuration, enforcement and recent history before implementation; decide how the new settings relate to existing `git.mode` without duplicating or weakening safeguards.
- Specify safe defaults, precedence and the meaning of administrator authorization; do not imply a setting grants remote permissions or overrides server-side protection. Keep force-push policy explicit.
- Enable contributors to configure their own AGit workflow and reuse a default topic, typically their Git username, for multiple issue or documentation updates in one pull request rather than opening one per issue. Target `main` via AGit; do not imply the updates reach `main` before the PR is merged. Verify repeated pushes update the intended PR and handle closed PRs safely.
- Reserve separate branches and pull requests for features and code changes; define how mixed changes are handled. Coordinate automated index updates with issue 1055.
- Cover configuration validation and representative admin and non-admin workflows with tests and documentation. Identify the authorized machine/role without treating self-assertion as remote permission; report server-side permission or protection blockers.
