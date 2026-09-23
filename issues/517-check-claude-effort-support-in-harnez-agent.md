# 517 — Check claude:* effort support in harnez agent

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `internal/subagent/driver.go` (`KnownModelEntries`: effort = provider != "claude"), `docs/Models.md` (2026-09-24 snapshot), `docs/studies/2026-09-24-model-research-claude.md`, [[497-harnez-agent-ignores-codex-tier-and-maps-claude-aliases-to-stale-model-ids]]

## Problem

`harnez agent models` shows EFFORT "no" for every `claude:*` row, and no `:med` variant
exists for them. The 2026-09-24 web research contradicted this with high confidence:
Anthropic documents low/medium/high effort for Sonnet 4.6+ and Opus 4.6+. The Claude
aliases now resolve to Sonnet 5 / Opus 5.x / Haiku 4.5, whose effort support is unverified.

## /goal

`claude:*` rows show EFFORT correctly, and where supported, `harnez agent` passes the
tier to the Claude CLI.

- Canary first: does the installed `claude` CLI accept an effort setting in the headless
  mode harnez uses (flag, env or settings), and does it change behaviour? Check per model
  (haiku likely no).
- Only then change the provider rule and add `:med` rows.

## Done when

- A canary result recorded in the ticket; code and `harnez agent models` match it; test
  covers the claude effort mapping.
