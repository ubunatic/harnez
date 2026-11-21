# 1253 — Bare model key whose provider is also an alias does not resolve (strata:<name>)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1252

---

## 1. Problem & Motivation

`ResolveModel("strata:qwen3.8-flash-next-iq2_xs")` (and `...-iq3_xxs`) fails with "unknown model
tier": `strata` is also a model alias, so the resolver reads the spec as `<alias>:<tier>`. The
spec key itself, as listed in `spec/agent.yaml`, cannot be passed to `--model`; only the tiered
form (`strata:qwen3.8-flash-next-iq2_xs:low`) works. Found by the spec-wide resolution test
written for 1252.

## 2. Technical Specification / Findings

- Resolver: `internal/subagent/driver.go` (`resolveModelIn`), alias lookup before provider:key.
- `TestResolveModel` skips the bare key when its provider is also an alias; remove that skip
  once fixed.

## 3. Implementation & Verification Plan

When a two-part spec's second part is not a known tier but `provider:key` is a spec key, resolve
it as that key at its default tier. Verify with `TestResolveModel` without the skip.
