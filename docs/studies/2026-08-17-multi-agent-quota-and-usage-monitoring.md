# Case Study: Multi-Agent Quota, Token & Session Usage Monitoring

**Date**: 2026-08-17  
**Scope**: Token, session, and rate-limit tracking across Claude Code, Antigravity (AGY), and OpenAI Codex CLI  
**Feature Issue**: [Issue 023: `harnez usage`](../../issues/023-usage-command-token-quota-tracking.md)  
**Status**: Implemented, verified, committed to `main` (`d74fce8`)  

---

## 1. Header & Context

Modern agentic engineering workflows frequently distribute tasks across multiple AI coding harnesses:
- **Claude Code** (`claude` CLI)
- **Google Antigravity** (`agy` / Antigravity CLI)
- **OpenAI Codex** (`codex` CLI)

Each tool enforces distinct rate-limiting windows (rolling 4h/5h session caps vs. 7-day weekly allowances) and uses different subscription tiers (Pro/Plus/Max/Consumer). Previously, checking remaining coding capacity required jumping across three separate interactive TUIs to run `/usage` or `/status`. The goal of this session was to build a single, non-destructive, zero-cost CLI monitor (`harnez usage`) that surfaces live quota buckets, local token volume, active models, and reset countdowns in a unified terminal card view.

---

## 2. Executive Summary

- Built **`internal/usage/`** and the Cobra CLI command **`harnez usage`** (with aliases `quota`, `tokens`, `stats`).
- **Zero-Cost & Non-Intrusive Extraction**:
  - **Claude Code**: Extracts lifetime token counters from `~/.claude/stats-cache.json` and polls live 5h/7d quota percentages via `GET https://api.anthropic.com/api/oauth/usage` using cached OAuth bearer tokens without launching interactive TUIs or spending billable tokens.
  - **Antigravity (AGY)**: Dynamically discovers the active loopback port of running AGY LanguageServer processes and invokes the Connect RPC endpoint `/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary` to track both **Gemini Models** (*Flash/Pro*) and **Claude & GPT Models** (*Opus/Sonnet/GPT-OSS*) pools.
  - **OpenAI Codex CLI**: Decodes unverified OAuth JWT claims from `~/.codex/auth.json` and queries live weekly rate limit status via `GET https://chatgpt.com/backend-api/wham/usage`.
- **Polish & Invariants**:
  - **Privacy-First**: Automatically masks user account emails (`u***l@gmail.com`) and never leaks private tokens or session UUIDs.
  - **TUI Box Geometry**: Terminal cards calculate block widths dynamically and close right borders (`│`, `┐`, `┘`) exactly 2 characters past the longest content line.
  - **Dim Wrapped Sources**: Displays an aligned, dimmed (`\033[2m`) `sources: ...` list under each box detailing all inspected local files and backend APIs.
  - **Unmanaged Claude Model**: Commented out `model: sonnet` in `config.yaml` to prevent `harnez apply` from downgrading users on Sonnet 5 to legacy aliases.

---

## 3. What Worked Well

1. **Subagent Delegation & Parallel Investigation**:
   - Initial scaffolding and data schema normalization was delegated to a subagent that researched local storage formats across `~/.claude/`, `~/.codex/`, and `~/.gemini/antigravity-cli/`.
   - A subsequent subagent cracked the live extraction mechanisms for AGY (local Connect RPC over Unix loopback sockets) and Codex (`wham/usage` with JWT session headers).
2. **Offline-First Resilience**:
   - Every collector gracefully falls back to local SQLite/JSON caches when offline (`--offline`), ensuring deterministic results with no network dependency.
   - **Correction (2026-08-18)**: this "graceful" framing overstated what actually happened on a *live* rate limit (HTTP 429) — a real one hit during multi-instance `--watch` testing and turned out to be silently dropped, not gracefully handled: `Session`/`Weekly` just went `nil` with nothing shown. See [Issue 031](../../issues/031-usage-quota-fetch-errors-silent.md), [032](../../issues/032-usage-watch-no-stale-fallback-on-fetch-failure.md), [033](../../issues/033-usage-shared-quota-cache.md) and [the follow-up study](2026-08-18-usage-quota-fetch-robustness-and-shared-cache.md) for the actual fix.
3. **Comprehensive Unit Tests**:
   - Built isolated table-driven tests for time duration formatting, number grouping, Unicode progress bar rendering, account email masking, and mock API response collectors.

---

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)

### 1. The Accidental Model Alias Downgrade
- **Issue**: Running `harnez usage` initially reported `Active Model: claude-sonnet-4-6` for Claude Code even when the user intended to use Sonnet 5.
- **Root Cause**: `internal/claude/apply.go` had a static alias map (`"sonnet": "claude-sonnet-4-6"`), and `config.yaml` explicitly specified `model: sonnet`. Running `harnez apply` wrote this fixed string into `~/.claude/settings.json`.
- **Fix**: Removed/commented out `model` from `config.yaml` so `harnez apply` leaves Claude Code's model setting unmanaged, allowing Claude Code to use its default or user-selected model.

### 2. Broken Tool Dependencies Interface in Concurrent Workspace
- **Issue**: Running `go run ./cmd/harnez usage` failed compilation because `internal/tools/voice_type.go` expected a `RunStdin` method on `Dependencies` that was missing in `internal/tools/tools.go`.
- **Root Cause**: An in-progress voice-typing feature branch had edited `voice_type.go` without completing the `Dependencies` struct definition.
- **Fix**: Implemented `RunStdin func(ctx context.Context, stdin string, name string, args ...string) error` in `tools.go` with standard `exec.CommandContext` execution.

### 3. Open TUI Right Borders & Overflowing Sources
- **Issue**: Initial terminal card rendering produced open-ended right borders (`┌───...`, `│ ...`, `└───...` with no closing right edge) and long unformatted source lines that broke terminal wrapping.
- **Fix**: Refactored `RenderText` in `internal/usage/usage.go` to compute visible Unicode rune length per block, format closed rectangular boxes with 2ch padding, and wrap `sources: ...` notes with hanging indentation.

---

## 5. Quality & Invariants Audit

| Area | Status | Evidence / Verification |
| :--- | :--- | :--- |
| **Module Isolation** | Pass | `internal/usage` is self-contained with no tight coupling to `internal/claude` or CLI runners. |
| **Data Privacy** | Pass | `MaskAccount` masks all user identity strings (`user@example.com` → `u***r@example.com`); no OAuth tokens or UUIDs exposed. |
| **Idempotency** | Pass | `harnez usage` is purely read-only and emits zero state changes or billable token consumption. |
| **Test Coverage** | Pass | `go test -v ./internal/usage/...` runs in <10ms with 100% pass rate across collectors and utilities. |
| **CLI Ergonomics** | Pass | Supports ANSI color/dim styling in interactive mode, `--offline` for pure local reads, and `--json` for machine scripting. |

---

## 6. Key Learnings & Evergreen Upstream

1. **Keep Model Configurations Unmanaged by Default**:
   - Coding harnesses evolve rapidly (Sonnet 3.5 → 3.7 → 4.6 → 5). Hardcoding short model aliases in repository configuration files creates silent downgrade footguns when developers switch models in their interactive sessions.
2. **Local Loopback RPCs as Quota Oracles**:
   - Modern AI desktop CLIs (like Antigravity) run background LanguageServer processes exposing structured Connect/gRPC endpoints on loopback. Probing running processes via `/proc` provides instantaneous, high-fidelity telemetry without internet roundtrips.
3. **Box Drawing Requires Strict Rune Counting**:
   - Multi-byte UTF-8 characters (like `█`, `░`, `┌`, `└`) distort string length if measured via `len()`. Always use `unicode/utf8.RuneCountInString()` when aligning terminal borders.

---

## 7. File & Commit Summary

- **Created**:
  - `issues/023-usage-command-token-quota-tracking.md` — Issue specification & extraction documentation
  - `internal/usage/types.go` — Normalized usage, quota, and model pool data structures
  - `internal/usage/util.go` — Email masking, duration formatting, progress bar rendering
  - `internal/usage/claude.go` — Claude Code local stats & live usage API collector
  - `internal/usage/agy.go` — Antigravity settings & LanguageServer Connect RPC collector
  - `internal/usage/codex.go` — OpenAI Codex JWT claims & `wham/usage` rate limit collector
  - `internal/usage/usage.go` — Multi-agent coordinator, closed box TUI & JSON renderers
  - `internal/usage/*_test.go` — Unit tests for usage subsystem
  - `scripts/canary-usage.sh` — Canary probe script
- **Modified**:
  - `config.yaml` — Commented out managed model override
  - `cmd/harnez/main.go` — Registered `harnez usage` command and aliases (`quota`, `tokens`, `stats`)
  - `internal/tools/tools.go` — Added `RunStdin` to `Dependencies`
- **Git Commit**:
  - `d74fce8` — `feat(usage): add multi-pool AGY & Codex quotas, dim wrapped sources, and unmanaged Claude model`
