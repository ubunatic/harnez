# 688 — Make agent delete all repeat-safe

**Status**: Open — M2: never-started Codex sessions stay delete-failed forever
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [Codex provider deletion](../internal/subagent/codex.go)

---

## 1. Problem & Motivation
After `harnez agent delete --all`, running it again can show the same sessions
again, including Codex sessions whose provider-side delete returned an error.
Repeated cleanup should converge instead of presenting the same agents on
every invocation.

## 2. Technical Specification / Findings
Make `delete --all` repeat-safe: sessions handled by one invocation should not
be listed again by the next invocation. Preserve a clear way to inspect or
retry unresolved provider deletions without repeating them as ordinary pending
sessions on every global delete.

## 3. Implementation & Verification Plan
**Goal**: Ensure repeated `harnez agent delete --all` runs do not show agents
already handled by an earlier run. Done when a second run skips prior entries,
including provider failures, while unresolved failures remain explicitly
recoverable and are not silently mistaken for confirmed provider deletion.
Stop and report if a provider cannot distinguish a deleted session from an
unresolved one.

## 4. Decisions & Pre-Work (2026-10-02)
- **Stop condition hit, resolved by user go:** `ClaudeDriver.Delete` and `AgyDriver.Delete`
  are no-ops returning nil, so no provider-side deletion is confirmed for them. They are
  reported as "removed from harnez only", never as provider-confirmed deletion.
- **User requirement 1:** `harnez agent delete --all --force` prints no "unrated latest turns"
  warning (today `printUnratedDeleteWarning` runs even with `--force`, `cmd/harnez/agent.go`).
  Without `--force` the warning and refusal stay as they are.
- **User requirement 2:** names deleted by one run never reappear in the next run or in the
  default `agent list`, for every provider. A Codex provider-delete failure keeps a record
  marked `delete-failed` (with the error) that is hidden from `delete --all` and the default
  list, shown only via an explicit recovery path (`delete --retry-failed`, and a list flag
  such as `--failed`); a successful retry removes it.

## 5. Delivery (2c4d4ec6)
- Root cause: `agent list` merged Codex rollout discovery without checking the deleted-session
  archive, so deleted sessions came back as external rows. Archived provider identities are now
  excluded from discovery.
- `--force` suppresses the unrated warning on `--name`, `--all` and `--all-completed`.
- Provider-delete failures persist as `delete-failed` (with the error), hidden from the default
  list and skipped by `delete --all` (one summary line); `list --failed` and
  `delete --retry-failed` are the recovery path.
- Claude/Agy: output prints only the deleted name and never claims provider-side deletion; no
  extra per-session notice was added, to keep `--all --force` quiet as the user asked.
- Verified: host `make test-q1` green; live `agent delete --name dev688 --force` left no
  rediscovered row in two `agent list --all-sessions` runs.

## 6. M2 (missing Codex sessions count as deleted) — Pre-Work
- Live finding after M1: 6 Codex sessions stay `delete-failed` and every `delete --all` prints
  "6 unresolved provider deletion(s) skipped". All have 0 tokens and v4 harnez IDs (e.g.
  `5f789df2-99ea-4795-8fab-42d9ca1ccc58`); no Codex rollout exists for them, so they never
  became Codex threads. `codex delete --force <unknown-uuid>` exits 1 with the same
  "Error: failed to delete session" as a real failure, so the exit code cannot tell them apart.
- Fix in `CodexDriver.Delete` (`internal/subagent/codex.go`): when no rollout file for the id
  exists under the Codex sessions dir (`$CODEX_HOME/sessions`, default `~/.codex/sessions`,
  files `rollout-*-<id>.jsonl`), return nil without calling codex (nothing to delete). Only a
  failing delete of an existing rollout stays `delete-failed`. Reuse the existing rollout
  discovery code if it already resolves that dir.
- Existing records: `delete --retry-failed` must then clear the 6 records; also let
  `delete --all` retry `delete-failed` records whose rollout is gone instead of skipping them,
  so the user never needs a second flag for this case.
- Tests: missing rollout -> nil and codex not started; existing rollout + codex error -> error;
  existing rollout + success -> nil; CODEX_HOME override honored.
- M2 delivered (e0ccbadf): `CodexDriver.Delete` returns nil when no rollout exists; `delete --all`
  retries Codex `delete-failed` records (deviation: all Codex ones, not only rollout-less ones, so a
  real persistent Codex failure is retried and reported each run). Verified live: host
  `make test-q1` green; `delete --retry-failed` cleared the 6 stuck records; `delete --all --force`
  run twice from a plain terminal printed `dev688m2` then nothing, no warning, no reappearance.
