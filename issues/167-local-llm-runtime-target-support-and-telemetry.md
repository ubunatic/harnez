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

---

## 5. Implementation Plan

Filed low-priority by the user, so this plan is a design skeleton to pick up later, not a queued
work item. It is written to be small enough that whoever picks it up can ship one runtime first and
stop there if local-LLM use never grows.

### Step 0 — Resolve the flag surface before writing any collector

Do not ship `--agent local`. `--agent` on `harnez usage` (`cmd/harnez/main.go:260`) already means
"filter to a specific agent (claude, agy, codex)" — an *identity* filter over collected agents, and
`rate`/`stats` use `--agent` for the same identity concept. A local runtime is another agent
identity, not a different flag concept, so the consistent move is:

- Register local runtimes as ordinary agent IDs in the existing collector registry
  (`internal/usage/`), e.g. `llamacpp`, `ollama`, `vllm`. Then `--agent ollama` works with zero new
  flags, `harnez usage` picks them up in the aggregate automatically, and `--watch`'s existing
  panels apply.
- Gate discovery on config, not on a flag: a `local_runtimes:` block in `config.yaml` listing
  `{id, kind, base_url}`. Nothing is polled unless configured, so users with no local server pay
  nothing and no localhost probing happens by default.

This also sidesteps the `[R]` remote-host toggle collision noted in §3 — that toggle stays about
*where* harnez collects from, unchanged.

### Step 1 — One collector interface, three thin readers

1. Define the reader contract next to the existing agent collectors in `internal/usage/`:
   `localRuntimeReader interface { Collect(ctx) (AgentUsage, error) }`.
2. Implement `internal/usage/llamacpp.go` first (`/health` then `/slots`): it is the only one of the
   three that directly reports per-slot context fill, which is the ticket's actual motivating
   metric. Ship this alone as v1.
3. `internal/usage/ollama.go` (`/api/ps`) second — reports loaded models and VRAM size, but not
   context fill, so it can only populate a coarser signal. Be explicit in the ticket/doc that Ollama
   gives less than llama.cpp here rather than faking a percentage.
4. `internal/usage/vllm.go` (Prometheus `/metrics`) last — needs a minimal text-format parser for
   `vllm:gpu_cache_usage_perc` / `vllm:num_requests_running`. Parse only the handful of named series
   needed; do not pull in a Prometheus client library.

### Step 2 — Mapping to the existing quota model

The existing `QuotaWindow` shape (`UsedPercent`, `RemainingPercent`, `ResetAt`, `DurationLeft`) is a
*time-window* abstraction. KV-cache fill has no reset time. Map it as a single window with
`UsedPercent` set and `ResetAt`/`DurationLeft` left nil — the renderer must already tolerate a
missing duration. Verify that against `formatAllUsageTableLine` before relying on it; if it does
not, this ticket depends on the same single-window alignment work as [[172]].

### Step 3 — The >50% warning

Reuse the existing spec-driven indicator styling (`internal/usage/indicatorsspec.go`) rather than
hardcoding a threshold colour in the watch renderer — this project's convention is spec-driven
`--watch` styling. Add the threshold as a spec value so it is tunable without a rebuild.

### Key Decisions / Tradeoffs

- **Agent IDs over a new flag**: the whole feature then rides existing filtering, aggregation, JSON
  output, and watch rendering. The tradeoff is that a local runtime shows up in "All Usage" next to
  cloud quota rows, where "100% used" means something categorically different (cache pressure, not
  exhausted quota). Mitigate with a distinct label, not a distinct code path.
- **Config-gated, never auto-probed**: avoids surprise localhost HTTP traffic from a monitoring
  tool.
- **Telemetry policy**: as §3 anticipates, polling an application's own HTTP API is not the
  vendor-GPU-CLI pattern that
  `docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md` bans. Record that judgement
  in the study/doc when this lands, so the precedent is explicit rather than re-litigated.

### Risks / Open Questions

- llama.cpp's `/slots` endpoint has been unstable across releases (it has been disabled-by-default
  and reshaped more than once). Pin the tested `llama-server` version in the collector's doc comment
  and degrade to `/health`-only on a parse failure rather than erroring the whole usage frame.
- Three runtimes is three fixture sets to keep current with no upstream contract. If local-LLM use
  stays low, shipping only the llama.cpp reader is the correct end state, not a partial one.

### Scope

**Medium** overall; **small** if scoped to Step 0 + the llama.cpp reader only, which is the
recommended first (and possibly only) increment.
