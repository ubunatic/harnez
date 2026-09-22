# 497 — harnez agent ignores Codex tier and maps Claude aliases to stale model IDs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: `docs/ModelAdvisoryEval.md`, `docs/HarnezAgentArchitecture.md` (model table), `spec/agent.yaml`

## /goal

`harnez agent --model codex:luna:med` runs Codex at medium effort, and `claude:sonnet` / `claude:opus`
run current models.

## Problem

Found on 2026-09-22 while setting up the model advisory eval:

1. **The tier is dropped for Codex.** `internal/subagent/codex.go:50` and `:232` build
   `codex exec --json --dangerously-bypass-approvals-and-sandbox -m <name> <prompt>` without
   the tier, so every tier runs at the user's `~/.codex/config.toml` `model_reasoning_effort`
   (`low` on the reference machine). `luna:low` therefore happens to work there, but `luna:med`
   and `luna:high` silently run at low, and on a machine with a different default `luna:low`
   runs at that default instead. Only the
   interactive path (`interactive.go:84`) passes `--effort`. `docs/HarnezAgentArchitecture.md`
   claims `--effort low|medium`, and the whole escalation ladder in `docs/AgenticLoop.md` relies
   on the tier.
2. **The Claude aliases are stale.** `internal/subagent/driver.go:72` maps `claude:sonnet` to
   `claude-3-7-sonnet-20250219` and `claude:opus` to `claude-3-opus-20240229`. They should use
   current aliases (`sonnet`, `opus`) or IDs from `spec/`, not hard-coded Go constants
   (`docs/Spec.md`).
3. **`terra` is not a known model.** It is missing from `modelAliases`, although
   `docs/Models.md` discusses it and it worked in the eval via `codex exec -m gpt-5.6-terra`.

## Plan

- Pass the tier to `codex exec` and `exec resume` as `-c model_reasoning_effort=<low|medium|high>`,
  mapping `med` to `medium`. Add a driver test that asserts the args.
- Move the model aliases into `spec/agent.yaml`, add `codex:terra`, and use the current Claude
  aliases. Keep `KnownModels` output stable.
- Canary: run one `harnez agent start --model codex:luna:med` turn and confirm the effort in the
  Codex session log.

## Pre-Work (lean sprint, 2026-09-22)

- Tier mapping: `low`→`low`, `med`→`medium`, `high`→`high`, passed as
  `-c model_reasoning_effort=<v>` on both `exec` paths (`codex.go:50`, `:232`) and on
  `exec resume` (`codexResumeArgs`). Assert the exact args in a driver test.
- If aliases move to `spec/agent.yaml`, update `spec/schemas/agent.schema.json` and keep Go
  free of duplicated values (`docs/Spec.md`). Update the model table in
  `docs/HarnezAgentArchitecture.md`.
- The developer skips the live canary (leaf workers never run `harnez agent`); the host runs
  it after the commit.
