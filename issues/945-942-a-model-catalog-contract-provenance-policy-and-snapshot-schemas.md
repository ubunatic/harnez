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

## M1 delivered (7bd68231, sol:low)

Shared definitions in `agent.schema.json`, new `models`, `model-policy`, `model-access`, `model-snapshot`
schemas, probe budget in `spec/agent.yaml`, 8 fixtures and `internal/modelcatalog/contracts_test.go`. Host
`make test-q1` green.

## M2 — Pre-Work / Required Refinements (reviewer r945, opus:low)

1. `fact`: per-source rules: `discovered` needs method, time and account (add `account`); `measured` needs
   `evidence` with `minItems: 1`; `web` needs a URL; non-empty evidence generally, or document why not.
2. Conflicts: every scalar fact (`relative_cost`, `api_cost`, `subscription_cost`, ...) can hold several facts
   or a `disputed`/`conflicts` marker (947 needs disputed). `facts` becomes a closed key set (typos rejected).
3. Access status `advertised|accessible|denied|unknown|stale`; per-observation `method`; coverage per harness
   and account. Fix `access-valid.yaml` (status `available` next to an auth-error fact).
4. Snapshot: `id`, `previous`, the effective merged catalog, `applied: bool`; add
   `model-change-report.schema.json` with rows `{category, model, field, before, after, sources, evidence,
   severity}` for 948.
5. Model ID string format with a pattern, used by policy `enabled` and the `models` keys; `kind: custom|patch`
   with identity required for custom; `version_pin` checked against identity where possible; patchable fields
   include `relative_cost` and `purpose`. Allow tier-less models (no forced tier enum value).
6. Document legacy `cost` → `relative_cost` mapping and `null` = clear (schema `description` or docs/Spec.md);
   add `$id` to `agent.schema.json` so references resolve without the test's custom loader.
7. Invalid fixture per edge case: custom without identity, negative price, 0 vs null cost, disable-all-but-N
   bad ID, bad pin, web without URL, discovered without method/account/time, measured without evidence, tier
   outside enum, bad `observed_at` format (settles whether `format` is enforced; if not, add a pattern).
   Tests assert a rejection-message substring (keyword + path), not only "invalid".
8. Drop the `maximum: 20` duplication of the spec value from the schema (or justify it as a hard ceiling).

## M2 delivered

- Facts now require non-empty evidence, web URLs, and discovered method/account/time; measured facts
  require evidence. Scalar fact keys hold arrays so conflicting observations remain available, with an
  optional `disputed` marker; the fact-key set rejects typos.
- Access observations distinguish advertised from accessible, require a method, and record coverage by
  harness and account. Snapshots now carry identity, predecessor, effective catalog, and applied state;
  change reports have a strict row schema.
- Policy IDs use a shared pattern, custom entries require identity, tier is optional, and patches cover
  relative cost and purpose. Tests check version-pin equality when identity is present; JSON Schema cannot
  compare sibling values. Invalid fixtures assert both the rejection keyword and schema path.
- `docs/Spec.md` and its source document the legacy cost mapping and null-clearing semantics. Schema IDs
  provide stable reference bases, and the test resolver uses those IDs without a hardcoded base URI. The
  `jsonschema-go` API still requires a loader to retrieve external embedded schema documents.
- Removed the schema's duplicate 20-call ceiling; the probe limit remains in `spec/agent.yaml`.
- Verification: `go test ./internal/modelcatalog ./internal/claude -count=1` passed; `make install`
  completed. The one `make test-q1` run failed at the root-doc copy check (fixed afterward) and an
  unrelated fatal map access in `internal/usage`; the full suite was not rerun.
