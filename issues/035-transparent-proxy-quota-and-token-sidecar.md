# 035 — Transparent Local HTTP_PROXY Sidecar for Zero-Wrap Rate Limit & Quota Interception

**Status**: Open  
**Category**: Architecture / Telemetry / Networking  
**Related**: [Issue 023: `harnez usage`](023-usage-command-token-quota-tracking.md), [Issue 030: Missing Local Token Counts](030-agy-codex-missing-local-token-counts.md), [Issue 034: Hook-Triggered Token Extraction](034-hook-triggered-token-extraction.md), [Study: Agent Telemetry](../docs/studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md)  

---

## 1. Problem & Motivation

While local file seeking ([Issue 034](034-hook-triggered-token-extraction.md)) solves token-count tracking for agents that write plaintext transcripts, it cannot capture **rate limits, remaining quota headers, or 5-hour rolling utilization**:
- Providers (Anthropic, OpenAI, Gemini) return rate-limit budgets (`anthropic-ratelimit-*`, `x-ratelimit-*`) exclusively in **HTTP response headers**.
- Agent CLI binaries strip or consume these headers internally and never expose them to hooks or log files.
- Manual proxy setups (e.g. running LiteLLM, mitmproxy, or Portkey manually) require users to manage local servers, handle TLS root certificate trust, and wrap CLI commands with custom environment variables or alias wrappers.

## 2. Goal: Zero-Wrap, Zero-Config Proxying Managed by `harnez`

`harnez` should manage the proxy lifecycle and configuration automatically so that standard `claude`, `agy`, and `codex` commands route through a local loopback sidecar with zero user intervention.

```text
┌────────────────────────────────────────────────────────────────────────────┐
│                        HARNEZ MANAGED SIDECAR                              │
│                                                                            │
│   ┌────────────────┐   HTTP_PROXY=localhost:8899    ┌──────────────────┐   │
│   │ Claude / Agy / │ ──────────────────────────────> │ harnez sidecar   │   │
│   │ Codex CLI      │                                 │ (loopback MITM)  │   │
│   └────────────────┘                                 └─────────┬────────┘   │
│                                                                │            │
│            ┌───────────────────────────────────────────────────┤            │
│            │ Passes through to upstream provider               │            │
│            ▼                                                   ▼            │
│   ┌──────────────────┐                               ┌──────────────────┐   │
│   │ api.anthropic    │                               │ Extract & update │   │
│   │ api.openai       │                               │ local usage      │   │
│   │ generativelanguage│                               │ cache on disk    │   │
│   └──────────────────┘                               └──────────────────┘   │
└────────────────────────────────────────────────────────────────────────────┘
```

## 3. How `harnez` Assists and Automates

### 3.1 Sidecar Lifecycle Management
- **Embedded Lightweight Forward Proxy**: Built into the `harnez` binary (`harnez daemon` or auto-started on demand by `harnez apply` / background service).
- **Zero Overhead**: Written in Go using `net/http` and `crypto/tls`, consuming <15MB RAM and adding <0.5ms latency to outbound LLM requests.

### 3.2 Automated Environment & Agent Configuration
Instead of requiring the user to run wrapped commands (`my-proxy claude ...`), `harnez apply` and `harnez init` inject configuration declaratively:
- **Claude Code**: `harnez apply` injects `HTTP_PROXY`, `HTTPS_PROXY`, and `SSL_CERT_FILE` (or `NODE_EXTRA_CA_CERTS`) directly into `~/.claude/settings.json` under `env` (which `harnez` already manages via `config.yaml`).
- **Antigravity (AGY)**: `harnez` sets proxy parameters in `.gemini/config` or shell profile exports.
- **Codex CLI / Shell**: `harnez apply` configures local shell environment blocks or `.env` files.

### 3.3 Ephemeral Loopback TLS & CA Management
- On first run, `harnez` generates a local CA keypair in `~/.local/share/harnez/ca.crt` (restricted to `0600` permissions).
- Automatically passes `SSL_CERT_FILE` / `NODE_EXTRA_CA_CERTS` via the agent's managed environment so Node.js and Go agent runtimes trust the loopback proxy seamlessly without modifying system-wide trust stores.

### 3.4 Telemetry Extraction & Cache Ingestion
The sidecar snoops only metadata (streaming headers and token chunk counts) without buffering or altering payload delivery:
- **Anthropic traffic (`api.anthropic.com`)**:
  - Headers: `anthropic-ratelimit-requests-remaining`, `anthropic-ratelimit-tokens-remaining`, `anthropic-ratelimit-input-tokens-reset`.
  - Body: captures final SSE chunk containing `usage` (`input_tokens`, `output_tokens`, `cache_read_input_tokens`, `cache_creation_input_tokens`).
- **OpenAI / Codex traffic (`api.openai.com`, `chatgpt.com`)**:
  - Headers: `x-ratelimit-remaining-tokens`, `x-ratelimit-remaining-requests`, `x-ratelimit-reset-tokens`.
  - Body: token usage breakdown from completion response.
- **Gemini / AGY traffic (`generativelanguage.googleapis.com`)**:
  - Quota and usage metrics parsed from Connect RPC / gRPC-Web frames.

Extracted data is flushed atomically to the shared flock cache (`~/.cache/harnez/usage.json`), instantly feeding `harnez usage` and `harnez usage --watch`.

## 4. Architecture & Security Invariants

1. **Strict Loopback Binding**: Proxy binds exclusively to `127.0.0.1` / `[::1]`.
2. **Pass-Through Non-LLM Traffic**: Any traffic not matching known LLM endpoint patterns (`api.anthropic.com`, `api.openai.com`, etc.) is tunneled via raw TCP `CONNECT` with zero inspection.
3. **No Key Storage**: The sidecar does not store or log authorization tokens/API keys; it inspects only usage headers and usage counters.
4. **Graceful Fallback**: If the sidecar is stopped, agent clients should bypass without blocking (configurable fail-open behavior).

## 5. Next Steps

1. Prototype the Go loopback MITM transport in `internal/proxy/`.
2. Update `internal/claude/apply.go` to inject proxy env entries when enabled in `config.yaml`.
3. Connect proxy metric emitter to `internal/usage/shared_cache.go`.

---

## Implementation Plan (revised after 2026-09-04 audit)

### Recommendation: do not build this yet — reduce it to a scoped canary first

This is the largest and riskiest item in the telemetry cluster (030/034/035),
and its unique value has shrunk since filing:

- **Token counts no longer need a proxy.** Claude already aggregates them
  (`~/.claude/stats-cache.json`, read at `internal/usage/claude.go:110`), and
  Codex now writes plain-JSON `token_count` events into
  `~/.codex/sessions/**/rollout-*.jsonl` (verified 2026-09-04, see revised plan
  on [030](030-agy-codex-missing-local-token-counts.md)). §3.4's body-sniffing
  half is therefore mostly redundant.
- **The remaining unique capability is rate-limit response headers**
  (`anthropic-ratelimit-*`, `x-ratelimit-*`) — real, genuinely unavailable
  elsewhere, but a narrow payoff for a MITM CA, TLS trust injection into three
  agent runtimes, and a daemon lifecycle.
- **`harnez usage` already fetches live quota** for all three agents through
  their own endpoints (`internal/usage/agy.go`, `codex.go`), giving percentage
  windows without touching the transport. The proxy would sharpen those numbers,
  not create them.
- **Cost profile is wrong for the payoff**: a local CA, `NODE_EXTRA_CA_CERTS` /
  `SSL_CERT_FILE` injection into managed env blocks, a long-running loopback
  daemon, and pass-through `CONNECT` tunnelling — every one of which is a way to
  silently break the user's *primary tools* if it misbehaves. Contrast with
  `harnez usage`'s current design, where the worst failure is a stale number.

### Step 0 — canary (the only work to do now)

Per `docs/other/Canary.md`, prove the payoff before building the machinery.
Standalone throwaway under `canary-proxy-headers/`, not in `internal/`:

1. Minimal Go `net/http` forward proxy on `127.0.0.1`, MITM for
   `api.anthropic.com` only, everything else raw `CONNECT` pass-through.
2. Run `claude` once with `HTTPS_PROXY` + `NODE_EXTRA_CA_CERTS` pointed at it.
3. Answer concretely:
   - Does Claude Code honour `HTTPS_PROXY` at all, and does it accept a custom
     CA via `NODE_EXTRA_CA_CERTS` (or does it pin certificates)?
   - Which `anthropic-ratelimit-*` headers actually arrive, with what values?
   - Do they say anything `harnez usage`'s existing quota fetch does not?
   - Measured added latency per request.
4. Repeat cheaply for `agy` and `codex` **only if** the Claude probe answers
   yes to certificate acceptance.

Write the answers into this ticket. If any agent pins certs, or the headers
duplicate what the quota endpoints already return, close the ticket with the
negative result recorded — that is the likely outcome and is worth knowing.

### Steps, only if the canary shows unique, valuable data

Ordered so each stage is independently useful and independently abandonable:

1. **`internal/proxy/`** — loopback forward proxy: `CONNECT` tunnelling,
   allowlisted MITM hosts, streaming pass-through with zero buffering of bodies
   (SSE must not be delayed). Header extraction only in stage 1; no body
   sniffing.
2. **CA management** — generate to `~/.local/share/harnez/`, `0600` on the key,
   `0644` on the cert, never install into the system trust store. Regenerate on
   expiry. Add an explicit `harnez proxy ca --path` for manual trust.
3. **`harnez proxy run|status|stop`** as an explicit, opt-in command group —
   **never auto-started by `apply`**. `docs/CLIDesign.md`'s scope separation and
   this repo's fail-open bias both argue against `apply` silently rerouting the
   user's LLM traffic.
4. **Env injection behind an explicit `config.yaml` opt-in** (default off), via
   the existing managed `env:` block in `internal/claude/apply.go`. Injecting
   `HTTPS_PROXY` by default would be the single most dangerous change this repo
   could ship.
5. **Sink** — write extracted rate-limit windows into the agent snapshot via
   `internal/usage/statecache.go` (the existing per-agent cache), **not** the
   ticket's proposed `internal/usage/shared_cache.go` / `~/.cache/harnez/usage.json`,
   neither of which exists.
6. **Tests** — proxy unit tests against an `httptest` upstream that emits known
   rate-limit headers and an SSE body: assert extracted values exactly, assert
   the body reaches the client byte-identical and unbuffered, and assert
   non-allowlisted hosts are tunnelled without inspection.

### Design decisions / tradeoffs

- **Opt-in daemon, never implicit.** Fail-open is not enough; a proxy that is
  on by default is on during every agent session, including when it is broken.
- **Headers only, no body inspection** in the first shippable stage — token
  counts are already solved locally, so the body path buys nothing and carries
  the entire risk of corrupting streamed responses.
- **No key/token logging, ever** — assert this in a test that feeds an
  `Authorization` header and greps every emitted artifact for it.
- **`statecache.go` over a new cache file**, consistent with 030 and 034.

### Risks / open questions

- Certificate pinning by any agent CLI kills the approach outright for that
  agent. Unknown until the canary runs.
- MITM of the user's own credentialed traffic is a meaningful security surface
  in a tool whose selling point is convenience. Even done correctly, the CA on
  disk is a new asset worth compromising.
- A stalled or crashed sidecar with `HTTPS_PROXY` still exported breaks every
  agent on the machine. Fail-open needs a concrete mechanism (short connect
  timeout plus a health check), not just a stated intent.
- Provider header names and semantics change without notice; extraction must
  degrade to "unknown", never to a wrong number displayed confidently.

### Scope

**Small** for the canary (a day's probe, and it may end the ticket).
**Large** for the full sidecar — the biggest item in the telemetry cluster, and
the one with the worst blast radius. Do 030 first, then 034's step 0, and only
then revisit this.

## Findings 2026-09-24 (host + luna advisor, read-only)

- No earlier interception test exists (searched issues, git log, docs, ~/.harnez logs).
- `agy` is a stripped Go ELF; no `usageMetadata` strings found in it. Proxy/CA support
  (`HTTPS_PROXY`, `SSL_CERT_FILE`) and certificate pinning are untested. No configurable API URL.
- Canary 1 (`agy -p "Reply with the single word: hi" --log-file`): the log has **no token counts**.
  It shows the architecture: the CLI starts a local language server (gRPC over HTTPS and HTTP on
  random localhost ports), which calls `https://daily-cloudcode-pa.googleapis.com/v1internal:`
  `streamGenerateContent?alt=sse` (2 calls for one prompt), `loadCodeAssist`, `fetchAvailableModels`.
  The log prints each URL with trace and response ID, and `Resolved proxyServerURL: ""`.
- Next, canary 2: one throwaway prompt through a local mitmproxy with `HTTPS_PROXY` and
  `SSL_CERT_FILE` set for that agy process only; check whether agy accepts it and whether the final
  SSE chunk has `usageMetadata`. Risk: agy's OAuth token passes the proxy; store nothing; check terms.
