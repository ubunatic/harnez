# 1267 — agent start admits a provider at 98 percent of its 5h quota because the gate refuses only at exhausted, and the 80 percent notice is advisory

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 508 (closed), 603, 1122, 485, 1183

---

## 1. Problem & Motivation

The hub's quota gate let a Claude worker start while its 5h window read 98%. The gate refuses only when quota reads as exhausted, so a worker can start far below the limit and then run out mid-task. Source: the hub-run report (orchestrator, 2026-11-22).

## 2. Technical Specification / Findings

- `internal/usage/statecache.go` (around lines 203-226): a provider is `exhausted` only when a window has `UsedFraction >= 1` with a future reset. Otherwise it is `stale` (reading too old or not fresh) or `available`.
- `internal/agent/core.go:188-206` (`RejectExhaustedQuota`): refuses only `State == "exhausted"`. `available` and `stale` both pass.
- An advisory 80% notice already exists: `spec/usage.yaml:23` (`agent_quota_warn_percent: 80`). `internal/usage/quota_notice.go` prints `quota: <provider> 5h NN% ...` to stderr for fresh windows at or above it. `cmd/harnez/agent_quota.go:14-25` (`warnModelQuota`) runs it on start (`cmd/harnez/agent_run.go:681`) and on resume (`:1178`). A fresh 98% reading therefore prints a warning and the start proceeds. This is the designed behaviour, not a bug in the notice.
- Stale readings never refuse, so a stale 98% reading would also start.
- Unverified: whether the hub saw that stderr line. The detached start path runs the child with its own stderr log (`cmd/harnez/agent_run.go:625-640`, `agent_async.go`), so the warning may not reach the orchestrator. Also unverified: whether the reading was fresh or stale when the worker started.
- Prior decisions: 508 (closed) added the exhausted-only gate and `--allow-exhausted-quota`; 1122 (closed) added `agent models --json` availability and `harnez models block`.

## 3. Implementation & Verification Plan

Owner decision first: whether a refusal threshold below 100% should exist, and for which windows (5h, weekly, or both). No value is proposed here; no source in this repo sets one. If adopted, put the value in `spec/usage.yaml` with a schema entry under `spec/schemas/usage.schema.json`, per `docs/Spec.md`, and never as a Go default.

Canary before any change: start a worker against a fresh reading at 98% and capture the start stderr and the detached child log. This shows whether the existing 80% warning reaches the orchestrator. Run the quota-wrapped test with `make test-q1` after any code change.
