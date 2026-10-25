# 943 — Canary: what claude, codex and agy report about available models

**Status**: Closed — agy lists models; claude and codex need a probe call (distinct errors unrecognized_model / model_not_found); codex models needs a TTY, unparsed; Anthropic list API unverified. Study docs/studies/2026-10-25-canary-model-access-discovery.md.
**Closed-By**: claude:opus-5-5, session 9ec5fca3
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Research
**Related**: 942 (epic), 941

---

/goal A canary study records, for each installed harness CLI (`claude`, `codex`, `agy`), how harnez can learn
which models this user can call: list commands or flags, config files, plan/tier info, the exact error for a
model the user lacks, and whether a cheap test call is needed. Each finding gives the command run and its real
output (trimmed). Stop and report when a probe would cost real quota beyond a few tiny calls or needs a login.
Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Scope

- Read-only probes: `--help`, model list commands, local config/cache files, version output.
- At most one tiny call per CLI with a model name that does not exist, to capture the access error.
- Note vendor API list endpoints (Anthropic, OpenAI, Google) only from docs; no API keys are used.
- Output: `docs/studies/<date>-canary-model-access-discovery.md` with `kind: canary`, and a short verdict per
  CLI: discoverable / partly / declare-only.
