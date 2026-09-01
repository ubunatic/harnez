# 167 — Local LLM Runtime Target Support & Telemetry (Ollama, llama.cpp, SGLang/vLLM)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[165-two-phase-architect-patch-harness-for-small-local-models]],
[[166-compact-local-llm-doc-profile-and-go-rune-width-invariants]] (same local-LLM development
push these two came from — this one is monitoring/telemetry rather than instruction/doc shaping,
so it's a separate concern, not the same profile mechanism), `internal/usage/usage.go`,
`internal/usage/watch.go`, `cmd/harnez/rate.go` / `cmd/harnez/stats.go` (existing `--agent` flag —
naming collision, see §3)

---

## 1. Problem & Motivation

External ticket received (source: pasted YAML ticket content — not yet triaged against harnez's
own priority/severity conventions; original fields were `type: feature`, `priority: medium`,
`target: harnez-usage-monitor`, re-scored below per harnez's own schema). User's own framing:
this is not high priority right now — "we don't do that much with local LLMs" today — and can be
built later; filed for tracking, not for near-term implementation.

`harnez usage` currently monitors cloud-provider quota windows (Claude/Codex/etc.). Local model
runners (llama.cpp's `llama-server`, Ollama, vLLM/SGLang) have a structurally different resource
concern: not a quota window but VRAM allocation, context-slot fill rate, and KV-cache pressure —
running a local model too close to its context window degrades output quality well before any hard
error occurs, and there's currently no visibility into that from `harnez usage`.

## 2. Requirements & Acceptance Criteria (from source ticket, unedited)

- [ ] Implement `--agent local` reader for `harnez usage`.
- [ ] Query metrics from:
  - `llama-server` (`/slots` and `/health`)
  - `ollama` (`/api/ps`)
  - `vLLM` / `SGLang` (`/metrics`)
- [ ] Display real-time context token usage vs. total window capacity.
- [ ] Add a warning indicator in `harnez usage --watch` when active context exceeds 50% of the
  allocated KV-cache window.

## 3. Notes / Open Questions

- **Naming collision**: `--agent` already exists as a flag on `harnez rate`/`harnez stats`
  (`cmd/harnez/rate.go`, `cmd/harnez/stats.go`), where it selects an `agent_id` like `claude` or
  `codex` for tool-feedback attribution — a different concept from "which local model runtime to
  poll." `harnez usage --watch` also already has its own `[R]` remote-host toggle
  (`internal/usage/watch.go`) for a different kind of "another target" selection. Whatever flag
  surface this eventually gets should be checked against both for consistency/collision before
  landing on `--agent local`.
- **Telemetry sourcing policy**: harnez's existing device-telemetry policy is kernel-standard-only
  — GPU/device metrics read procfs/sysfs, never a vendor CLI/SDK (`nvidia-smi`/`rocm-smi` were
  removed; see `docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md`). Polling
  `llama-server`/`ollama`/`vLLM` HTTP endpoints is a different category (an application-level API
  the local model server itself exposes, not a vendor GPU-vendor CLI wrapping driver internals), so
  it likely doesn't conflict with that policy — but worth an explicit call-out when this is actually
  scoped, since it's the same general area (device/runtime telemetry sourcing) that policy governs.
- Three different runtimes (`llama-server`, Ollama, vLLM/SGLang) with three different APIs (custom
  `/slots`+`/health`, `/api/ps`, Prometheus-style `/metrics`) means this is really three small
  readers behind one interface, not one reader with three flags — worth designing the
  runtime-detection/selection story explicitly when this is picked up.

## 4. Scope Note

Filed for tracking only, per user's explicit low-priority framing. No implementation planned until
local-LLM usage in this project grows enough to justify it.
