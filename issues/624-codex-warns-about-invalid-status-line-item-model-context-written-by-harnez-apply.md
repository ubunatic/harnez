# 624 — Codex warns about invalid status line item model-context written by harnez apply

**Status**: Closed — delivered in 5100c72e: model-context dropped and stripped from existing configs; verified in ~/.codex/config.toml
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #141

---

/goal `harnez apply` writes only status line items the installed Codex accepts, removes
`model-context` from existing configs it wrote, and Codex starts without the warning; stop and
report when blocked on a user decision or denied permission.

## 1. Problem & Motivation

Every Codex session (seen via `harnez agent start -i`, codex-cli 0.158.0) shows:

```text
Warnings · 1 of 1 · Warning
  Ignored invalid status line item: "model-context".
```

`internal/codex/statusline.go` hardcodes `model-context` in `statusLineItems` (added in 50912dd0),
and `ApplyStatusLine` writes it into `~/.codex/config.toml` `[tui] status_line`. Current Codex no
longer accepts that item (renamed or removed).

## 2. Technical Specification / Findings

- `ApplyStatusLine` only appends missing items and keeps existing ones, so dropping the item from
  the list alone will not remove it from configs already written. Apply must also strip items
  Harnez once added and now knows are invalid (not user-chosen ones).
- Canary: get Codex's current valid item list (docs, `/statusline` picker, or source) and check
  whether `model-context` has a replacement.
- Per `docs/Spec.md`, consider moving the item list to the embedded spec (see `/respect`, #617).

## 3. Implementation & Verification Plan

- Test: apply on a config containing `model-context` removes it and keeps user items.
- `make install`, `harnez apply`, start codex: no warning.
