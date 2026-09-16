---
title: Agent Session Token and Quota Trackability Status
weight: 98
---

# Agent Session Token and Quota Trackability Status

**Date**: 2026-09-16  
**Scope**: Token accounting, per-step / tool usage breakdown, full session lifecycle metrics, and weekly subscription quota tracking across Google Antigravity (AGY), Anthropic Claude Code, and OpenAI Codex CLI.  
**Related Docs & Studies**:  
- [studies/2026-08-17-multi-agent-quota-and-usage-monitoring.md](2026-08-17-multi-agent-quota-and-usage-monitoring.md)  
- [studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md](2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md)  
- [studies/2026-08-30-usage-panel-integration-and-the-agy-collector-cliff.md](2026-08-30-usage-panel-integration-and-the-agy-collector-cliff.md)  
- [CLIDesign.md](../CLIDesign.md)

---

## 1. Executive Summary & Capabilities Matrix

As agentic coding workflows mature, tracking token consumption and capacity windows requires visibility across three granularities:
1. **Per-Step & Tool Usage**: Fine-grained cost attribution per model turn, thinking phase, and tool invocation.
2. **Full Session Metrics**: Aggregate input, output, cache, and cost metrics spanning from session start to shutdown.
3. **Weekly Quota & Subscription Limit History**: Rolling quota capacity curves (5-hour bursts, 7-day allowances) and long-term token volume records.

### Current Support Matrix

| Metric Dimension | Antigravity (AGY) | Claude Code | OpenAI Codex |
| :--- | :--- | :--- | :--- |
| **1. Per-Step / Tool Tokens** | ❌ Omitted in logs/hooks (requires tokenizer / proxy) | ✅ Exact per-turn tokens in JSONL logs | ⚠️ Session rollouts without tool breakdown |
| **2. Full Session Tokens** | ❌ Not stored locally by client binary | ✅ Cumulative in `stats-cache.json` | ✅ Aggregated from rollout logs |
| **3. Live 5h / 7d Quotas** | ✅ Real-time % & reset timer via `harnez usage` | ✅ Real-time % & reset timer via `harnez usage` | ✅ Real-time % & reset timer via `harnez usage` |
| **4. Raw Weekly Token Total** | ⚠️ Quota % tracked; raw token sum requires proxy/parser | ✅ Exact cumulative lifetime & model tokens | ✅ Exact cumulative lifetime & model tokens |

---

## 2. Granularity Breakdown

### 2.1 Step-Level & Tool Usage Token Trackability

* **Google Antigravity (AGY)**:
  * **What is written locally**: AGY writes event trajectories into `~/.gemini/antigravity-cli/brain/<conv-id>/.system_generated/logs/transcript.jsonl` and SQLite summary databases (`conversation_summaries.db`). Each step contains `step_index`, `type` (`USER_INPUT`, `PLANNER_RESPONSE`, `GENERIC`), `tool_calls`, `content`, and `thinking`.
  * **What is missing**: Step records **do not contain raw token counts** (`input_tokens`, `output_tokens`, `cached_tokens`, `thinking_tokens`).
  * **Hook Limitations**: Agent lifecycle hooks (`hooks.json`) provide execution context (`conversationId`, `workspacePaths`, `transcriptPath`, `modelName`, `stepIdx`, `invocationNum`), but **do not include token payloads or HTTP response headers**.
  * **Measurement Pathway**: Token counts per step/tool call can only be approximated locally via offline tokenizers (e.g. `tiktoken` / Gemini tokenizer libraries) or captured verbatim via a loopback proxy.

* **Claude Code**:
  * **Native Support**: Claude logs every message turn directly to `~/.claude/projects/<slug>/<session-id>.jsonl` with exact token usage metadata:
    * `input_tokens`
    * `output_tokens`
    * `cache_creation_input_tokens`
    * `cache_read_input_tokens`
  * Tool inputs and return payloads are recorded with their surrounding message tokens.

* **OpenAI Codex**:
  * Step events are recorded within rollout sessions in `~/.codex/sessions/`, but individual tool executions lack isolated sub-token counters.

---

### 2.2 Full Session Metrics (Start $\rightarrow$ Work $\rightarrow$ Stop)

* **Antigravity (AGY)**:
  * AGY maintains session start/stop timestamps, conversation identifiers, and workspace bindings in `history.jsonl` and `conversation_summaries.db`.
  * However, the client binary does not accumulate or persist session-wide token totals upon exit.
  * **Harnez Tracking**: `harnez usage` displays conversation counts (e.g., `478 conversations`) and active model information, but session token rollups require offline parsing of transcripts or HTTP proxy interception.

* **Claude Code**:
  * At session teardown, Claude automatically aggregates token counts into `~/.claude/stats-cache.json` broken down by individual model (e.g., `claude-sonnet-5`, `claude-opus-5`, `claude-fable-5`).

* **OpenAI Codex**:
  * `harnez usage` computes total tokens dynamically by scanning and aggregating all session rollout files in `~/.codex/sessions/`.

---

### 2.3 Weekly Quota Data & Subscription Limit History

* **Live Quota Windows (Burst vs. Rolling Weekly)**:
  * Supported across all three platforms via `harnez usage` and cached in `harnez-quota-cache.json`:
    * **AGY**: Discovers separate quota buckets for *Gemini Models* (5-hour limit and 7-day limit) and *Claude and GPT models* (7-day limit) along with exact `reset_at` timestamps.
    * **Claude Code**: Queries Anthropic OAuth endpoints for 5-hour burst session % and 7-day weekly % with reset countdowns.
    * **OpenAI Codex**: Reads 5-hour and 7-day rolling window percentages and reset timers.

* **The Quota Capacity Denominator Problem**:
  * Upstream LLM providers (Google, Anthropic, OpenAI) return **percentages and reset timestamps**, but **do not disclose raw token capacity ceilings** (e.g., "83% remaining of a secret 100M token weekly cap").
  * Real absolute weekly token totals must therefore be calculated from client-side token logs rather than upstream quota API responses.

* **Historical Tracking & Trends**:
  * `harnez usage history` logs periodic snapshots of window percentages over time, enabling historical visualization of burst and weekly drain rates.

---

## 3. Recommended Roadmap & Next Steps

1. **Transcript Token Estimator (`harnez assess` / `internal/usage/agy.go`)**:
   * Implement an offline token counter for AGY `transcript.jsonl` logs to provide estimated session token totals and fill the AGY token display gap in `harnez usage`.
2. **Hook-Triggered Ingestion**:
   * Use `PostInvocation` and `Stop` hooks in AGY to immediately index completed steps without polling disk loops.
3. **Transparent HTTP Proxy Sidecar**:
   * For 100% wire-accurate token accounting and rate-limit header extraction across closed binaries, support an optional loopback proxy (`harnez proxy` / `harnez daemon`).
