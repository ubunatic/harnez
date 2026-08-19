# Study: AI Agent Telemetry — Lifecycle Hooks, Transcript Digging, and HTTP Proxy Sidecars

**Date**: 2026-08-19  
**Scope**: Telemetry, token tracking, and quota monitoring across Claude Code, Google Antigravity (AGY), and OpenAI Codex CLI  
**Related Issues**: [Issue 023](../../issues/023-usage-command-token-quota-tracking.md), [Issue 030](../../issues/030-agy-codex-missing-local-token-counts.md), [Issue 034](../../issues/034-hook-triggered-token-extraction.md), [Issue 035](../../issues/035-transparent-proxy-quota-and-token-sidecar.md)  
**Status**: Completed research and comparative analysis  

---

## 1. Context & Motivation

As agentic coding CLI tools (Claude Code, AGY, OpenAI Codex) proliferate, developers face two distinct monitoring requirements:
1. **Token & Cost Accounting**: Measuring input tokens, output tokens, cached token reads/writes, reasoning tokens, and cumulative financial cost per turn, session, and project.
2. **Quota & Rate-Limit Tracking**: Monitoring rolling window allowances (5-hour burst caps, weekly tier budgets) and countdowns to capacity reset.

Traditionally, extracting this telemetry required either:
- **Digging into local client files**: Parsing proprietary JSONL logs, scanning `stats-cache.json`, or reverse-engineering undocumented SQLite databases with binary Protobuf payloads (`~/.gemini/antigravity-cli/conversations/*.db`).
- **Polling remote OAuth endpoints**: Periodically querying endpoints like `api.anthropic.com/api/oauth/usage` or `chatgpt.com/backend-api/wham/usage`, which can introduce rate limits (`HTTP 429`) or coordination overhead across multiple terminal instances.

This study explores using **AI agent lifecycle hooks** (`hooks.json`, `~/.claude/settings.json`), evaluates industry practices among existing AI monitors (LiteLLM, Helicone, Portkey, Langfuse, ccusage, rtk), and establishes what is possible and what remains impossible across Claude, AGY, and Codex.

---

## 2. Industry Landscape: How AI Monitors Track Telemetry

A survey of current AI monitors and proxy solutions reveals four primary architectural patterns:

| Architecture | Representative Tools | Strengths | Weaknesses |
| :--- | :--- | :--- | :--- |
| **1. HTTP Gateway / Proxy** | LiteLLM, Helicone, Portkey, Cloudflare AI Gateway | Full access to raw HTTP request/response payloads, streaming chunks, and rate-limit headers (`x-ratelimit-*`, `anthropic-ratelimit-*`). Zero agent codebase modification. | Requires network routing configuration (`HTTP_PROXY`, base URL overrides) and TLS termination/CA setup for HTTPS. |
| **2. Log / Transcript Parsing** | ccusage, Claude Usage Dashboard, `harnez usage` (local stats) | Non-invasive; passive reads from disk; captures exact model outputs and session trees if logged. | Fragile against internal schema changes; files may encode data in opaque formats (e.g. AGY Protobuf blobs in SQLite); zero visibility into HTTP quota headers. |
| **3. SDK / Tracing Instrumentation** | Langfuse, LangSmith, OpenLLMetry (OTel) | Deep semantic understanding (agent call trees, tool inputs/outputs, step durations, eval scores). | Intrusive; requires source code instrumentation; impossible on closed-source compiled CLI binaries (Claude Code, AGY, Codex) unless plugin hooks exist. |
| **4. CLI Output / Transport Proxies** | rtk (Rust Token Killer), CLI wrappers | In-line token compression, command output filtering, process-level accounting. | Intercepts standard I/O rather than upstream LLM wire protocol. |

---

## 3. Platform Capabilities & Hook Analysis

### 3.1 Claude Code
- **Hook Mechanism**: Configured via `hooks` in `~/.claude/settings.json` or `.claude/hooks/` (`PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `Stop`).
- **Hook Payload**: Receives event context (command lines, tool names, parameters, session identifiers).
- **Telemetry Limits in Hooks**:
  - **No Token Counts**: Stdin does not contain token metrics (`input_tokens`, `output_tokens`, cache stats).
  - **No Quota Headers**: Rate-limit headers from Anthropic API responses are stripped before hooks run.
- **Transcript Access**: Full per-turn token breakdowns are appended to `~/.claude/projects/<slug>/<session-id>.jsonl` and aggregated in `~/.claude/stats-cache.json`.

### 3.2 Google Antigravity (AGY / Cortex)
- **Hook Mechanism**: Configured via `hooks.json` (`PreInvocation`, `PostInvocation`, `PreToolUse`, `PostToolUse`, `Stop`).
- **Hook Payload**: Receives `conversationId`, `workspacePaths`, `transcriptPath`, `artifactDirectoryPath`, `modelName`, `stepIdx`, `invocationNum`.
- **Telemetry Limits in Hooks**:
  - **No Direct Token Payload**: Hook JSON payloads do not include raw token counts or cost metadata.
  - **No Quota Data**: Gemini server-side quota states are not routed to hooks.
- **Transcript Access**: `transcriptPath` is passed directly in the hook payload, pointing directly to `<workspace>/.gemini/.../transcript.jsonl`. At `PostInvocation` or `Stop`, a hook can instantly read the exact step line without directory searching.

### 3.3 OpenAI Codex CLI
- **Hook Mechanism**: Minimal declarative hook support in CLI.
- **Telemetry Limits**:
  - Does not provide a generic `hooks.json` lifecycle dispatch.
  - Rate-limit headers (`x-ratelimit-*`) are processed internally for TUI status and discarded.
- **Local Storage**: `~/.codex/` does not store plaintext transcript JSONL on standard installations; session telemetry is locked in internal application state.

---

## 4. Key Findings: What is Possible vs. Not

1. **Pure Hooks Cannot Extract Quota or Rate Limits**:
   - HTTP response headers (e.g., Anthropic rolling 5-hour utilization, OpenAI request/token remaining buckets) are handled in the HTTP client layer of the agent binary and are **never** forwarded to lifecycle hooks.
2. **Pure Hooks Cannot Directly Provide Token Counts**:
   - Neither Claude nor AGY includes token counts in the hook stdin JSON.
3. **Hybrid Hook-Triggered File Seek is Ideal for Per-Turn Tokens**:
   - Using `PostInvocation` / `Stop` hooks as **event-driven triggers** eliminates polling. The hook receives the exact `transcriptPath` and line offset/index, enabling instantaneous, zero-poll token ingestion.
4. **HTTP Proxy Sidecars are the Only Path for Universal Zero-Wrap Rate Limit & Quota Interception**:
   - A local loopback HTTP proxy intercepting outbound LLM traffic is the only mechanism that reliably captures live rate-limit headers and exact wire token counts across all CLI agents without reverse-engineering local cache files or handling opaque SQLite blobs.

---

## 5. Architectural Comparison

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                             AGENT TELEMETRY                                 │
├───────────────────────┬────────────────────────────┬────────────────────────┤
│ Approach              │ Token Volume & Costs       │ Quotas & Rate Limits   │
├───────────────────────┼────────────────────────────┼────────────────────────┤
│ Pure Lifecycle Hook   │ ❌ Not in hook payload     │ ❌ Not in hook payload │
├───────────────────────┼────────────────────────────┼────────────────────────┤
│ Transcript Polling    │ ⚠️ High disk I/O / lag     │ ❌ Missing from disk   │
├───────────────────────┼────────────────────────────┼────────────────────────┤
│ Hook-Triggered Seek   │ ✅ Immediate, zero-polling │ ❌ Missing from disk   │
├───────────────────────┼────────────────────────────┼────────────────────────┤
│ Loopback HTTP Proxy   │ ✅ 100% wire accuracy      │ ✅ Exact HTTP headers  │
└───────────────────────┴────────────────────────────┴────────────────────────┘
```

---

## 6. Recommendations for Harnez

1. **Adopt Hook-Triggered File Seek for Local Token Ingestion ([Issue 034](../../issues/034-hook-triggered-token-extraction.md))**:
   - Deploy lightweight hook scripts via `harnez apply` that trigger on `PostInvocation` (AGY) and `Stop` (Claude).
   - On trigger, the hook script reads the new step from `transcriptPath` and pushes token numbers to the local `harnez` shared metrics store.
2. **Build an Automated Local HTTP_PROXY Sidecar ([Issue 035](../../issues/035-transparent-proxy-quota-and-token-sidecar.md))**:
   - Implement `harnez daemon` / `harnez proxy` as an optional, transparent loopback proxy.
   - Have `harnez apply` configure standard environment variables (`HTTP_PROXY`, `HTTPS_PROXY`, `SSL_CERT_FILE`) or client configs automatically, eliminating the need for custom CLI wrappers or user-managed proxy infrastructure.
