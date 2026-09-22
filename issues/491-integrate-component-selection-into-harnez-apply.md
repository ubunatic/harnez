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

## Pre-Work (lean sprint, 2026-09-22)

Advisors terra:low and sonnet agreed on the cut; code claims below were checked on HEAD.

**Milestones** (one commit each, `(issue 491 M<n>)`):

1. **M1 — config keys, no behavior change.** `Config.ComponentNames` (`components:`),
   `Command.Requires` (`requires:`) in `internal/claude/config.go`; `requires: [telemetry]` on
   `tool-feedback-protocol` in `config.yaml`; retarget `internal/components` to read them and
   drop `components.SkillRequires` (`internal/components/components.go:138`). Existing
   `internal/components` tests stay green; add a config round-trip test.
2. **M2 — flag and schema gating.** `apply --components <names>` (presets and names mix),
   resolved once and shared by `apply`/`diff`/`status`. `ensureTelemetrySchema()` runs before
   `claude.OpenConfig` today (`cmd/harnez/main.go:521-523`): reorder so the selection exists
   first, then gate it on `telemetry`. `ApplyAllVariant` (`internal/claude/apply.go:869`) takes
   the resolved set. Tests: unfiltered `settings.json` byte-identical to before;
   `--components docs-only` on an empty target creates no telemetry DB.
3. **M3 — single-write settings removal (review seam).** Fold removal into
   `buildSettingsDoc` (`apply.go:118`) / `applyMerge` (`apply.go:79`): a key is removed only if it
   is in `managedSettingsKeys` (`apply.go:98`), never because a value is nil alone; strip any
   sentinel before marshal so no `null` lands in the file. `buildSettingsDoc` omits `hooks`
   when empty: under a selection, write `hooks` explicitly so managed hooks get removed.
   `diffSettingsJSON` (`apply.go:203`) must apply the same removal. Delete
   `components.PruneSettings`. Tests: full → docs-only → full equals a fresh full apply
   byte for byte; user keys (permissions entries, `mcpServers`, a user hook, a user status
   line) survive docs-only; `DiffAll` is clean after a selected apply.
4. **M4 — skills, Codex/AGY, retire the MVP.** Generalize the rate-feedback removal branch
   (`apply.go:921`) to unmet `requires:`, removing `SKILL.md` *and* the skill's resource files
   (reuse `skillTargets`). Codex/AGY: `Apply` with `telemetry`, `Remove` without; their
   `Remove` is already ownership-aware (02c9a0b), don't re-implement it. `DiffAll`
   (`apply.go:1210`) must check Codex/AGY under a selection with the same resolved set, not
   skip them because the filtered config blanked their targets. Move the `internal/components`
   tests onto `claude.ApplyAll*`/`DiffAll`, delete the package and `scripts/canary-components`.
   `scripts/smoke-test.sh` passes; `make install`.

**Out of scope:** dispatch-mode clamping (493), project-level selection (494), persisting the
selection to local config (§8.10, needs `LoadLocalConfig` moved). The MVP's "open" comment
about `requires: [agents]` on sprint skills is stale (§8.9): don't add it.

## M1 delivered (f9ef081) — M2 Pre-Work / Required Refinements

M1 added `components:`/`requires:`, set `requires: [telemetry]` on `tool-feedback-protocol`,
and replaced `SkillRequires` with `requiredComponents(s.Requires)`. No assertions changed.

1. **Validate `requires:` names.** An unknown name (typo `telemtry`) becomes a component that
   no set contains, so the skill silently disappears under every selection, including `full`.
   Reject unknown `requires:` and `components:` names once, where the selection is resolved,
   with an error naming the skill and the bad value. Test both.
