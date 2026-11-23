# 1277 — Add taia as driver for local models instead of pi when taia CLI is available

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `spec/agent.yaml`, `internal/agent/`

---

## Goal

`/goal` Add `taia` as an agent driver for local models and prefer it over `pi` when the `taia` CLI is available on `PATH`, falling back to `pi` when `taia` is absent, with unit tests and spec/doc updates. Stop and report if blocked on a user decision (e.g., `taia` CLI invocation contract or model-ID mapping) or denied permission. On a loop, hard blocker, or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

Local models in Harnez (e.g., Strata entries in `spec/agent.yaml`) currently route through the `pi` driver. On machines where the `taia` CLI is installed, Harnez should be able to drive local models through `taia` instead of `pi`, while keeping `pi` as the fallback when `taia` is not available.

## 2. Scope & Notes

- Re-verify live `internal/agent/` driver registration, `spec/agent.yaml` local-model definitions, and `taia` CLI flags/session interface before implementing.
- Prefer `taia` for local models when `taia` is available in `PATH` (or explicitly selected), and fall back to `pi` otherwise.
- Keep configurable defaults/driver metadata in `spec/` per `docs/Spec.md`.

## 3. Verification

- Unit tests covering driver selection (`taia` present vs absent), command construction, and fallback to `pi`.
- `HTO=0 make test-q1` passes.
