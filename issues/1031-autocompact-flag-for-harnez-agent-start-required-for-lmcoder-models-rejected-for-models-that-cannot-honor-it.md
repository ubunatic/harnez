# 1031 — Autocompact flag for harnez agent start, required for lmcoder models, rejected for models that cannot honor it

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1023, 1030, 969, 869, 987

---

## Ask (owner, 2026-10-29)
When agents call `harnez agent` with lmcoder models, they MUST give the context size, either directly or by
setting `--autocompact` (a flag adopted from Claude). A custom `--autocompact` works for every agent and model
that can support it; passing it for a model that cannot honor it fails with a clear error.

## Why
Local lmcoder models have small, changing windows (32k, 64k, 96k planned). Runs that relied on defaults ended
with `length` or looped through compactions (1023, 1024, lmcoder 263). Making the caller state the budget turns
a silent misfit into an explicit choice.

## Open questions
- Flag shape: what Claude's autocompact setting takes (percent of the window, a token limit, on/off) and which
  form harnez adopts; a separate flag for the context size, or one flag for both.
- Per provider, what can honor it: Pi (`compaction.reserveTokens`/`keepRecentTokens`, 1023), Codex
  (`-c model_auto_compact_token_limit`, used in 987), Claude (its autocompact setting), agy (869). Which fail.
- How it relates to the spec defaults in `agent.compact_thresholds` (969) and the `harnez apply` override (1023),
  and to the looked-up window from 1030 (does a known live window satisfy the "must specify" rule?).
- Resume: does the value persist with the session.

## Acceptance
- `harnez agent start` with an lmcoder model and neither a context size nor `--autocompact` fails with a message
  that names both options.
- `--autocompact` applies on every provider that supports it (verified live once per provider) and fails
  before launch on one that does not.
- Values live in `spec/` with a schema where they are defaults (`docs/Spec.md`); `docs/CLIDesign.md` read first.

/goal Add the flag and the lmcoder requirement, or stop and report when the flag shape or a provider's support
needs an owner decision. Circuit breaker: on a loop or no progress, clear the goal and write a failure report.
Re-check live code first; written 2026-10-29.
