# 023 — `harnez usage`: Unified Token, Session & Quota Status Command

**Status**: Closed — resolved in `48a2585` (`--watch` live view) and `6c82176` (`--summary` view, quota-fetch robustness), 2026-08-18  
**Category**: Feature / CLI Tooling  
**Command**: `harnez usage` (aliases/related: `quota`, `tokens`, `stats`)  

---

## Context & Motivation

Agentic coding workflows frequently switch between different AI coding harnesses:
- **Claude Code** (`claude`)
- **Antigravity** (`agy` / Antigravity CLI)
- **OpenAI Codex CLI** (`codex`)

Each assistant tracks token consumption, session-level limits (rolling 4h/5h windows), and weekly rate limits using different subscription tiers (Claude Pro/Team/Max, Antigravity/Gemini subscriptions, ChatGPT Plus/Codex tiers).

Previously, checking available capacity required jumping into each interactive TUI separately and running `/usage` or `/status`. A unified `harnez usage` CLI command provides an instant, cross-agent summary in the terminal.

---

## Real-World TUI Dump References

### 1. Claude Code (`/usage` dump)
```text
  Session

  Total cost:            $0.0000
  Total duration (API):  0s
  Total duration (wall): 5s
  Total code changes:    0 lines added, 0 lines removed
  Usage:                 0 input, 0 output, 0 cache read, 0 cache write

  Current session
  ██████████████████████████████████████████████████ 100% used
  Resets 11:20pm (Europe/Berlin)

  Current week (all models)
  ████████                                           16% used
  Resets Aug 22, 7pm (Europe/Berlin)
  +50% weekly limits promo through Aug 19 · clau.de/cc-50-promo
```

**Key Data Points:**
- Session token breakdown: `input`, `output`, `cache read`, `cache write`.
- Session cost: `$0.0000` (or computed API cost).
- Session window limit: `% used` + exact reset time & timezone (`Resets 11:20pm (Europe/Berlin)`).
- Weekly limit: `% used` (all models) + reset date/time (`Resets Aug 22, 7pm (Europe/Berlin)`).
- Active promotions/modifiers (e.g. `+50% weekly limits promo`).

---

### 2. Antigravity / AGY (`/usage` dump)
```text
└ Models & Quota

  Account: user@example.com

GEMINI MODELS
  Models within this group: Gemini Flash, Gemini Pro

  Weekly Limit Remaining
    [██████████████████████████████████████████████████] 99.95%
    100% remaining · Refreshes in 165h 5m

  Five Hour Limit Remaining
    [██████████████████████████████████████████████████] 99.71%
    100% remaining · Refreshes in 4h 52m


CLAUDE AND GPT MODELS
  Models within this group: Claude Opus, Claude Sonnet, GPT-OSS

  Weekly Limit Remaining
    [██████████████████████████████████████████████████] 100.00%
    Quota available

  Five Hour Limit Remaining
    [██████████████████████████████████████████████████] 100.00%
```

**Key Data Points:**
- Account identity (`user@example.com`).
- Model Group breakdown:
  - **Gemini Models** (Gemini Flash, Gemini Pro)
  - **Claude and GPT Models** (Claude Opus, Claude Sonnet, GPT-OSS)
- Tiered Windows:
  - **5-Hour Limit Remaining**: `% remaining` + countdown (`Refreshes in 4h 52m`).
  - **Weekly Limit Remaining**: `% remaining` + countdown (`Refreshes in 165h 5m`).

---

### 3. OpenAI Codex CLI (`/status` dump)
```text
╭────────────────────────────────────────────────────────────────────────────────╮
│  >_ OpenAI Codex (v0.147.0)                                                    │
│                                                                                │
│ Visit https://chatgpt.com/codex/settings/usage for up-to-date                  │
│ information on rate limits and credits                                         │
│                                                                                │
│  Model:                gpt-5.6-sol (reasoning low, summaries auto)             │
│  Directory:            ~/projects/harnez                                       │
│  Permissions:          Workspace (Ask for approval)                            │
│  Agents.md:            AGENTS.md                                               │
│  Account:              user@example.com (Plus)                                 │
│  Collaboration mode:   Default                                                 │
│  Session:              <session-uuid>                                          │
│                                                                                │
│  Weekly limit:         [░░░░░░░░░░░░░░░░░░░░] 0% left (resets 22:44 on 20 Aug) │
╰────────────────────────────────────────────────────────────────────────────────╯
```

**Key Data Points:**
- Client version (`v0.147.0`).
- Active model & configuration (`gpt-5.6-sol (reasoning low, summaries auto)`).
- Account tier & identity (`user@example.com (Plus)`).
- Active session ID (`<session-uuid>`).
- Weekly limit: `% left` + reset timestamp (`resets 22:44 on 20 Aug`).

---

## Research Findings: Storage & Extraction Mechanisms

### 1. Claude Code (`~/.claude/`)
- **Local Token & Activity Store**: `~/.claude/stats-cache.json` tracks lifetime token counts by model (`inputTokens`, `outputTokens`, `cacheReadInputTokens`, `cacheCreationInputTokens`, `costUSD`), daily model breakdown (`dailyModelTokens`), total sessions (`totalSessions`), and total messages (`totalMessages`).
- **Auth & Subscription Tier**: `~/.claude/.credentials.json` contains `claudeAiOauth` (`subscriptionType`: "pro"/"max", `rateLimitTier`: "default_max", `accessToken`, `expiresAt`).
- **Live Quota API**:
  - `GET https://api.anthropic.com/api/oauth/usage` with headers `Authorization: Bearer <accessToken>`, `anthropic-client: claude-code/<version>`, `anthropic-beta: oauth-2025-04-20`.
  - Returns `five_hour` and `seven_day` utilization percentages and ISO reset timestamps without spawning an interactive TUI or consuming billable LLM tokens.
  - Fallback: When offline or rate-limited (HTTP 429), local cached totals from `stats-cache.json` are returned cleanly.

### 2. OpenAI Codex CLI (`~/.codex/`)
- **Auth & Tier Discovery**: `~/.codex/auth.json` contains OAuth JWT tokens (`id_token`, `access_token`). Unverified JWT payload inspection reveals:
  - `https://api.openai.com/auth.chatgpt_plan_type`: subscription tier ("plus", "pro", "team", "free").
  - `https://api.openai.com/profile.email`: account identity (automatically masked for privacy).
  - Expiration timestamp `exp`.
- **Config & Model State**: `~/.codex/config.toml` specifies active `model` (e.g. `gpt-5.6-sol`) and `model_reasoning_effort`.
- **Live Rate Limit Endpoint**:
  - `GET https://chatgpt.com/backend-api/wham/usage` with headers `Authorization: Bearer <access_token>`, `ChatGPT-Account-ID: <account_id>`, `User-Agent: codex`.
  - Returns live `rate_limit` containing `primary_window` (`used_percent`, `limit_window_seconds`, `reset_after_seconds`, `reset_at`), plan type, and credits balance.
- **Session & Thread Databases**: `~/.codex/state_5.sqlite` and `logs_2.sqlite` track local session threads and tokens used.

### 3. Antigravity / AGY (`~/.gemini/antigravity-cli/`)
- **Settings & Model**: `~/.gemini/antigravity-cli/settings.json` stores active model configuration (e.g. `Gemini 3.7 Flash (Low)`).
- **Auth & Token**: `~/.gemini/antigravity-cli/antigravity-oauth-token` contains oauth bearer token object and `auth_method` ("consumer").
- **Live Quota RPC**:
  - Running AGY processes host a Connect RPC server on loopback.
  - Calling `POST /exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary` returns full model pool groups ("Gemini Models" and "Claude and GPT Models") with their 5-hour and weekly quota buckets (`remainingFraction`, `resetTime`).
- **Session State & Telemetry**: `~/.gemini/antigravity-cli/conversations/*.db` contains local conversation SQLite trajectory databases, while logs under `~/.gemini/antigravity-cli/log/` record account email and session activity.

---

## Implementation Summary

- **Package `internal/usage/`**:
  - `types.go`: Normalized schemas (`AgentUsage`, `ModelGroup`, `QuotaWindow`, `TokenBreakdown`, `UsageSummary`).
  - `util.go`: Privacy-first email masking (`MaskAccount`), ANSI/Unicode progress bar renderer (`RenderProgressBar`), duration and integer formatting.
  - `claude.go`: Claude Code collector (local credentials, stats-cache, and live OAuth usage endpoint).
  - `codex.go`: Codex CLI collector (JWT claim extraction, auth mode, config parsing, and live `wham/usage` rate limit queries).
  - `agy.go`: Antigravity collector (settings, token, conversation counts, and local LanguageServer Connect RPC quota extraction for Gemini and Claude/GPT model pools).
  - `usage.go`: Multi-agent collection coordinator and renderers (`RenderText`, `RenderJSON`).
  - `collectors_test.go`, `util_test.go`, `usage_test.go`: 100% passing unit tests.
- **CLI Commands & Flags**:
  - `harnez usage` (Aliases: `quota`, `tokens`, `stats`).
  - Flags:
    - `--json`: Machine-readable JSON output for scripting, tmux, and status bars.
    - `--agent <id>`: Filter output to a specific agent (`claude`, `agy`, `codex`).
    - `--offline`: Bypass live remote API queries and read local cache files only.
- **Canary Script**: `scripts/canary-usage.sh` probes local directories and validates non-interactive extraction.

---

## Next Steps / Future Enhancements

1. Integrate status bar snippets (e.g., tmux or Waybar modules) consuming `harnez usage --json`.
2. Support API pay-per-token spending caps (OpenAI / Anthropic developer API keys) in addition to subscription tiers.

## Update 2026-08-18: `--watch` live view added

Added `harnez usage --watch` (`internal/usage/watch.go`): a live-refreshing, btop-style grid of
per-agent panels with per-panel toggle keys (embedded in each panel's own title bar), a tokens/min
sparkline, and a `--interval` flag (default 60s, floored at 30s) so the live view can't be pointed
at live quota APIs faster than the data changes. Also fixed a real bug in the AGY collector
(`internal/usage/agy.go`): the Antigravity CLI listens on two loopback ports and the original
port-discovery only TCP-probed before picking one, silently dropping quota data about half the
time — now tries every candidate port until the RPC actually answers.

Full writeup, including a three-theory debugging saga on a terminal-rendering bug in the watch
view (stale-content bleed + gutter off-by-one, not the glyph-width issues initially suspected):
[docs/studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md](../docs/studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md).

Discovered along the way and tracked separately: AGY and Codex have no local token-count source
to populate the watch view's token toggle from — see
[Issue 030](030-agy-codex-missing-local-token-counts.md).

**This work is uncommitted as of this update** — see the study doc §6.

## Update 2026-08-18 (continued): `--summary` one-shot view + quota-fetch robustness

Added `harnez usage --summary` (`-s`): prints the same compact `--watch`-style grid as a single
frame and exits — for scripting or a quick glance, so the command now has three reporting modes
(full detail, live `--watch`, one-shot `--summary`).

While testing `--summary` against a live `--watch` session, hit a real `HTTP 429` from
`api.anthropic.com/api/oauth/usage` — caused by two concurrent `harnez` processes (a leftover
`go run` from testing plus a real `--watch`) independently polling the same account. That surfaced
three latent robustness gaps, tracked and closed as [Issue 031](031-usage-quota-fetch-errors-silent.md)
(fetch failures were completely silent), [Issue 032](032-usage-watch-no-stale-fallback-on-fetch-failure.md)
(`--watch` blanked a panel on a transient failure instead of keeping the last good value), and
[Issue 033](033-usage-shared-quota-cache.md) (no cross-process coordination, so concurrent instances
multiplied the request rate). Design discussion and the flock-based fix are written up in
[docs/studies/2026-08-18-usage-quota-fetch-robustness-and-shared-cache.md](../docs/studies/2026-08-18-usage-quota-fetch-robustness-and-shared-cache.md).

Committed as `6c82176` (code) and `fc74910` (website docs).

