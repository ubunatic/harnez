---
kind: sprint
ticket: 968
date: 2026-10-26
---
# Lean sprint 968: skill and tool gating

## Goal

Cut the fixed context overhead of harnez Claude sessions (about 33k tokens interactive: skills 5.3k, system
tools 12.9k, deferred 15k) by disabling unused skills, making orchestration skills invoke-only, and removing
unneeded MCP tools.

Owner basis (2026-10-26): files only, no MCP unless really needed, no cloud connectors, missing access becomes
a `harnez <verb>`; default is to disable.

## Team

| Role | Session | Model | Turns |
|---|---|---|---|
| Searcher A (harnez skill sources) | s968a | luna:med | 1, 53s |
| Searcher B (Claude Code settings docs) | s968b | luna:med | 1, 44s |
| Refiner | r968 | sol:med | 1, 1m40s |
| Advisor (decision review) | a968 | agy:argon:low | 1 |
| Developer | d968 | sol:med | 4 (plan, M1, M2, M3) |

## Findings

- `skillOverrides` in `~/.claude/settings.json` hides User, Built-in and, proven by canary here, claude.ai-synced
  skills (`xlsx` row gone). `disableBundledSkills` covers bundled ones.
- `disable-model-invocation: true` removes a skill from the model listing and keeps `/name`.
- Bare-name `permissions.deny` removes a tool definition; scoped rules only block calls (refiner corrected
  searcher B). `deniedMcpServers` removes an MCP server's tools; `disableClaudeAiConnectors` removes connectors.
- harnez already wrote `skillOverrides` from `config.yaml` `preset_skill_overrides`, but only under
  `--debloat`/`--debloat-preset`; plain `apply` skips it.
- `claude -p "/context"` works non-interactively; print-mode totals differ from interactive ones, so rows are
  the reliable check (issue 348 accounting shift: System tools rose 521 → 4k after skills were hidden).

## Decisions

- D1: invoke-only via source frontmatter for harnez-sourced skills (`epic`, `reverse-sprint`, `lean-sprint-v1`),
  plus `user-invocable-only` overrides for all gated skills. The advisor overturned the host's override-only
  proposal because plain `apply` and non-Claude harnesses never see the overrides.
- D2: unused system skills `"off"`; synced ones included after the canary proved it works.
- D3: no `--tools` allowlist; keep `ToolSearch`; deny the harnez and codebase-memory MCP servers and disable
  claude.ai connectors in the preset.
- D4: the owner note answered the open gating question: disable or gate, keep `tool-feedback-protocol`.
- D5: debloat stays opt-in; changing plain `apply` is out of scope.

## Milestones

| Milestone | Commits | Result |
|---|---|---|
| M1 canary evidence | b6dd7670 | overrides and MCP denial proven on Claude Code 2.1.310 |
| M2 config, schema, MCP denial, tests | 18448876, ef5b703f | `make test-q1` pass; applied live |
| M3 docs | 07f944c9 | README, `docs/CLIDesign.md` |

## Verification

- Host: diff review, `go test ./internal/claude/` ok, live settings checked (18 `"off"` overrides, 2 denied
  servers, connectors off). The Claude Docs connector dropped from the host session right after apply.
- Before/after (print mode): Skills 5.3k → 1.7k; MCP tool rows 33 → 0; total 15.5k → 14k.

## Observations

- `harnez agent start --stream stats` piped through `sed -n '/\[reply/,$p'` lost the agy and sol replies
  ("final message was already shown above"); the host had to read `response` from the session JSON.
- `harnez issues close` refused while untracked tickets were in the index; the host committed them with the close.
- Net saving in print mode is smaller (1.5k) than the per-row drops, because deferred MCP tools were not loaded
  before; the interactive saving is not yet measured.
