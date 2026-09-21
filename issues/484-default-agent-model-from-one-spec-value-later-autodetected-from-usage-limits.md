# 484 — Default agent model from one spec value, later autodetected from usage limits

**Status**: Open

**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: #479 (epic, design §2.1), #481, #144, #291, `docs/Spec.md`

---

## 1. Problem & Motivation

`agent start` requires a model argument. Most calls want the cheap default.
The default must live in one place (see `docs/Spec.md`: Go code must not
duplicate spec values), and later it should adapt to remaining quota.

## 2. Technical Specification

- Phase 1: default `codex:luna:low` when `--model` is omitted, defined once in
  the spec/config source and read from there; `harnez agent models` marks it as
  the default. Also accept bare aliases such as `luna` when unambiguous.
- Phase 2 (later, separate follow-up): choose the model from available 5h and
  weekly limits via `internal/usage`, falling back to the phase 1 default when
  usage data is missing. Must be explainable: the stream header prints
  `model=… (default|autodetected: <reason>)`.
- An explicit `--model` always wins. #144 (model-selection policy) decides which
  model fits which task type; this ticket only supplies the fallback.

## 3. Implementation & Verification Plan

- Tests: omitted `--model` resolves to the default; explicit value wins; bare
  alias resolution and ambiguity error; `models` output marks the default.
- Phase 2 gets its own ticket when the usage-limit source is stable.
