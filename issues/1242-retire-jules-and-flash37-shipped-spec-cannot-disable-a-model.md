# 1242 — Retire Jules and flash37: shipped spec cannot disable a model

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Cleanup
**Related**: 1241 (model matrix), 1185, 1218 (Jules bugs, moot once retired)

---

## 1. Problem & Motivation

The owner asked on 2026-11-17 to disable `jules:jules` and `agy:flash37`. Jules sessions ended on
2026-11-11 and Google shuts it down on 2026-12-31 (spec note). flash37 is marked legacy in favour of
flash38. Both are not in any pick chain, but `harnez agent models` still lists them as usable.

## 2. Technical Specification / Findings

- Disabling exists only in the user policy (`models.yaml`, `enabled: false` or `default_enabled`,
  `modelcatalog/catalog.go` `IsEnabled`). The shipped `spec/agent.yaml` has no disable flag, so a
  spec-only change cannot disable a model for every user.
- Removing Jules touches about 20 Go files (`rg -l 'jules' --type go`), the `/jules` skill and
  `provider_modes`.

## 3. Implementation & Verification Plan

- Decide: a spec-level `disabled: true` model field (schema, `IsEnabled`, shown as disabled in
  `agent models`), or removal. Recommendation: the flag for flash37, removal for Jules after
  2026-12-31 with the skill and its tickets.
- Verify: `harnez agent start --dry-run --model flash37` refuses with "disabled".
