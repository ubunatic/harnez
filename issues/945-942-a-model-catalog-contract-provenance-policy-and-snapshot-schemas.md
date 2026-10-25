# 945 — 942 A: model catalog contract, provenance, policy and snapshot schemas

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 942 (epic, see ## Architecture and ## Decision), 941, 946, 947, 948, 949

---

/goal The model formats exist as strict, versioned JSON schemas with fixtures: model identity (harness, provider,
exact model/version, tier; aliases separate), fact provenance `{value, source, observed_at, evidence}`, cost
fields, user policy (`models.yaml`), access observations and snapshots; the shipped `spec/agent.yaml` still
validates unchanged or with a documented migration. Or stop and report on a format choice. Circuit breaker: on
a loop or no progress, clear the goal and write a failure report here.

## Scope (code: specs, schemas, fixtures, schema tests)

- Extend `spec/schemas/agent.schema.json` (around :91) with shared model definitions; new schemas for policy,
  access and snapshot. Strict: unknown keys rejected.
- Costs per epic decision 4: `cost` stays relative; subscription (plan, currency, period, quota) and API
  (per-million input/output/cache, effective date) facts; unknown is null, never 0.
- Policy per epic: field-level patches, custom entries, version pins, `default_enabled: false` plus explicit
  enabled IDs, arrays replace, explicit clearing.
- Probe budget lives in spec (epic decision 5): max calls per update (about 20), per-call token cap, timeout.
- Tests: valid and invalid fixtures for each schema; legacy catalog compatibility.
- Read `docs/Spec.md` first. No loader changes here (that is 941).
