# 491 — Integrate component selection into harnez apply

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 490 (design, MVP in `internal/components`), 489, 492, 493, 494, 495, `docs/HarnezComponents.md` §8

## /goal

Move the component selection MVP (`internal/components`) into `internal/claude` and the
`apply` command, so `harnez apply --components docs-only` (and a `components:` config key)
works end to end with one settings write and consistent `diff`/`status`.

## 1. Problem & Motivation

The 490 MVP wraps `claude.ApplyAllVariant` from outside because the main files were off
limits. That leaves gaps listed in `docs/HarnezComponents.md` §8.10:

- `components:` (config) and `requires:` (skills) keys are not parsed.
- `apply` has no `--components` flag; `ensureTelemetrySchema` runs unconditionally.
- Removal is a second pass that rewrites `settings.json` after apply.
- `components.SkillTargets` duplicates the unexported `skillTargets`.
- Skills removed for unmet `requires:` lose `SKILL.md` only; resource files stay.
- `diff`/`status` do not know the selection; under a selection `DiffAll` skips the Codex
  and AGY files because the filtered config blanks their targets.

## 2. Technical Specification

- `Config.ComponentNames` (`components:`), `Command.Requires` (`requires:`); drop the
  `components.SkillRequires` stand-in table and set `requires: [telemetry]` on
  `tool-feedback-protocol` in `config.yaml`.
- `apply --components <names>` (presets and names mix); resolve once, shared by `apply`,
  `diff`, `status`.
- Gate `ensureTelemetrySchema` on `telemetry`.
- Fold removal into `buildSettingsDoc`/`applyMerge` (nil value = remove if harnez-owned),
  so one write does both; always write `hooks` under a selection.
- Generalize the rate-feedback skill removal branch to unmet `requires:`.
- Codex/AGY: apply with `telemetry`, `Remove` without (both are ownership-aware since
  02c9a0b); make `DiffAll` check them under a selection.
- Keep `internal/components` tests as the acceptance suite, retargeted at `claude.ApplyAll*`,
  then delete the MVP package and `scripts/canary-components` (or turn the canary into
  `harnez apply --components … --plan`).

Decisions (§8.9): sprint skills do not get `requires: [agents]`; dispatch is an opt-in
mode (493), and a disabled `agents` component clamps the effective mode to `native`.
Project-level selection belongs to `init` (494), not this ticket.

## 3. Implementation & Verification Plan

- [ ] Config keys, flag, shared resolution
- [ ] Single-write settings removal; requires-driven skill removal
- [ ] Selection-aware `DiffAll`/status, including Codex/AGY
- [ ] Unfiltered apply output byte-identical to before (existing test pattern)
- [ ] Each preset boots on an empty target; presets compose by union
- [ ] `scripts/smoke-test.sh` passes; `make install`
