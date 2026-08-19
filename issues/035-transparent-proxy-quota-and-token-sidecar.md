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
