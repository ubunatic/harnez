# 1052 — Safely refresh the Pi background task extension independently

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 1050, 1051; `internal/claude/pi_background.go`, `docs/PiAgent.md`

## Problem and goal

`make install` updates the CLI but not the globally installed Pi task extension. The runner is managed through the broader `skills` apply selection; an earlier apply unexpectedly removed other managed configuration, and a direct copy would desynchronize its ownership ledger. No supported Pi-only update was found in read-only discovery. Consequently the installed global copy still predates 1051.

/goal Provide and verify a safe, narrowly scoped way to update the globally managed Pi background-task extension and its ownership ledger without touching unrelated managed or user configuration, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with a resume point.

Re-check current installer, CLI design, recent history and actual installed state before work. Preserve drift and unrecorded-file protections; test idempotence, ownership ledger consistency and unrelated-file preservation in isolated homes, then verify the installed runner with a real Pi canary. Do not overwrite user configuration or run a broad apply without reviewing its effects.

## Decision

Use an additive, narrowly scoped `harnez pi background-tasks update` command, subject to live code validation. Reuse the existing ledger-aware Pi install preflight and activation ordering; retain configured target and `PI_CODING_AGENT_DIR` handling. It must not call broad `ApplyAll` or remove unrelated managed items. This keeps `apply` global and `init` project-only and avoids changing existing flag semantics. Initial expected write set: `cmd/harnez/` command wiring/tests, `internal/claude/` narrow installer entry point and isolated-home tests, and `docs/PiAgent.md`; update after developer inspection. Test successful update, idempotence, ledger digests, modified/unrecorded refusal, recovery and unrelated-file preservation. Canary in an isolated home first; never overwrite drift in the actual global installation.

## Review pre-work

Review of `4d994a6c`: strict-mode refusal currently says to run broad `apply`; make its remediation specific to the targeted update. Strengthen refusal tests to assert the correct reason, including drifted manifest, symlinked parents, special files, unsafe ledger entries and leftover temp files, and prove unrelated configuration remains unchanged. Investigate and document interrupted temp-file recovery and the lack of concurrency locking/recheck; do not silently overwrite concurrent user edits. The isolated Pi RPC canary proved loading only, not a model-issued task. Before actual global update, verify ledger and installed target, back up outside Pi discovery, avoid concurrent apply/update, and verify idempotence and real task lifecycle afterwards.
