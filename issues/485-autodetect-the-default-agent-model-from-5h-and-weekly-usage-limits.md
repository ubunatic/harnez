# 485 — Autodetect the default agent model from 5h and weekly usage limits

**Status**: Open

**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: #484 (phase 1, closed), #479 (epic), #144, `internal/usage`, `spec/agent.yaml`

---

## 1. Problem & Motivation

Phase 1 of #484 gives `harnez agent start` a fixed default model
(`default_model` in `spec/agent.yaml`, currently `codex:luna:low`). When the
5h or weekly limit of a provider is nearly exhausted, a fixed default keeps
dispatching to a provider that is about to refuse work.

## 2. Technical Specification

- When `--model` is omitted, choose the default from the remaining 5h and
  weekly quota reported by `internal/usage`; fall back to the spec default when
  usage data is missing, stale or unreadable.
- The choice must be explainable: the streaming header prints
  `model: <spec> (default)` today; it becomes
  `model: <spec> (autodetected: <reason>)` when the usage source changed the
  outcome, for example `codex 5h limit 92% used, picked claude:haiku:low`.
- Selection rules live in the spec (`spec/agent.yaml`), not in Go: an ordered
  preference list plus the usage thresholds that demote an entry.
- An explicit `--model` always wins. #144 stays the place that decides which
  model fits which task type; this ticket only changes the fallback.

## 3. Implementation & Verification Plan

- Start only after the usage-limit source is stable and cheap to query (it must
  not slow down `agent start` noticeably; cache the reading).
- Tests with a fake usage reader: below threshold keeps the spec default, above
  threshold demotes to the next entry, missing data falls back, explicit
  `--model` bypasses detection, the header reason text is exact.
