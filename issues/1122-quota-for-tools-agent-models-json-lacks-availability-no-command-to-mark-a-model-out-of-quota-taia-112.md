# 1122 — Quota for tools: agent models --json lacks availability; no command to mark a model out of quota (taia 112)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 603, 924, 992, taia#112

---

/goal Give tools (taia) a machine-readable per-model "can start / quota state / reset" view and a way to mark a
model out of quota until a reset time that harnez honours; or stop and report when blocked on a user decision.

## 1. Problem & Motivation
taia (harnez/taia#112) wants to show, per model, whether the owner can use it and how much quota is left.
Today (checked 2026-11-05 on b8b6475e):
- `harnez agent models` (text) shows a per-model quota column (`available`, `exhausted (resets 3h30m)`,
  `stale (...)`, `unknown (...)`, `cmd/harnez/agent_cmds.go:203`) and a startable `yes/no` column, but
  `--json` rows carry only spec, model, cost, roles, use and source: no quota state, reset or startable flag.
- `harnez usage --json` `.agents[]` gives per **provider** `session`/`weekly` windows (`remaining_percent`,
  `reset_at`), `authenticated`, `stale`, `last_refreshed`; no per-model quota. For agy, `session`/`weekly` are
  null even while agy reports "Individual quota reached" for claude-sonnet-5-5.
- No command marks a model exhausted after a 429. The only record is the collector snapshot
  (`internal/usage/statecache.go`), plus `--allow-exhausted-quota` to override.

## 2. Plan
1. `agent models --json`: add `quota` {state, age_s, reset_at} and `startable` per row, the same values as the
   text columns.
2. `harnez models block <spec|provider:model> --until <time|duration> [--reason]` / `unblock`: a local,
   expiring record that `CachedProviderQuotaAvailabilities` treats as exhausted; shown in `agent models`.
   Decide: should agent start/resume auto-record it on a known quota error (agy "Individual quota reached",
   codex usage-limit)?
3. Tests: JSON fields match text; a block expires at its reset; start refuses a blocked model unless
   `--allow-exhausted-quota`.
