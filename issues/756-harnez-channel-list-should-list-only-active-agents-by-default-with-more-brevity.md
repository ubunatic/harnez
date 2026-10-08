# 756 — harnez channel list should list only active agents by default, with more brevity

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: issues/754-add-filtering-and-projection-flags-to-harnez-channel-list-live-names-ids-with-overview-first-design.md

---

/goal Make the default `harnez channel list` show only active agents in a brief overview, or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
`harnez channel list` still prints stale sessions by default. Issue 754 added `--live` as an opt-in filter, so the default output still includes historical Codex threads and endpoints that are no longer running. Users and agents read the default output first, so stale rows add noise. The default output is also more verbose than it needs to be.

## 2. Technical Specification / Findings
- `cmd/harnez/channel.go:117` applies the live filter only when `--live` is set. Without it, all discovered endpoints and Codex sessions are listed.
- `cmd/harnez/channel.go:181` marks rows as `live`. The default output should show only those rows.
- `internal/channel/registry.go:94` and `:142` already prune dead PIDs for the endpoint registry. Codex discovery (`channel.DiscoverCodexSessions`) is not yet checked for liveness; verify this before changing it.
- Open question: whether a Codex thread counts as active when its PID is gone but its thread was recently updated. Decide with the user, or keep the PID check as the only criterion.
- Brevity: the default table should carry only the columns needed to pick a target (`NAME`, `PROVIDER`, `ID`, short `STATUS`). Keep the detail hint footer.

## 3. Implementation & Verification Plan
- Make the default list show live entries only. Keep `--json` and the `--names`/`--ids` projections working. Decide whether `--all` should bring back stale rows; it is a reasonable escape hatch.
- Trim the default table columns and keep the detail hint footer.
- Verify against the current code by listing with a stale registry entry and a dead Codex thread present, and confirming neither appears by default.
- Update the `channel list` tests in `cmd/harnez` and `internal/channel` for the new default.
- Run `make install` after the Go change.
