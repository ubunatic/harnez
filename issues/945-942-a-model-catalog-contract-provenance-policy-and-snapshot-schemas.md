# 945 — 942 A: model catalog contract, provenance, policy and snapshot schemas

**Status**: Closed — M1 7bd68231 schemas+fixtures, M2 ed17b565 provenance/conflicts/access/snapshot/change report, M3 3cd14dca review fixes (rule-specific fixtures, 0-vs-null rule, ValidatePolicyPins in production, disputed and report formats documented). Host make test-q1 green after M3.
**Closed-By**: claude:opus-5-5, session 9ec5fca3
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
  Host `make test-q1` after ed17b565: green.

## M3 — Pre-Work / Required Refinements (reviewer r945b, opus:low)

1. `contracts_test.go:34` `discovered-without-time`: `observed_at` is required for every fact, so the case fails
   for the general reason. Make it fail only through the discovered rule (assert `required` + the field and the
   discovered branch path), or drop it.
2. `agent.schema.json:38`: the `measured` evidence rule duplicates the general `minItems: 1`. Remove it, or
   give `measured` a stricter rule that a fixture proves is needed.
3. Add an invalid fixture for a negative `api_cost`.
4. Settle 0 vs null: `docs/other/Spec.md:97` says zero is a valid measured value while the schema rejects 0 for
   `relative_cost`. State one rule per cost field and make doc (source + root copy) and schema agree.
5. Move the version-pin check (`validatePolicyPins`, `contracts_test.go:77`) into production code in
   `internal/modelcatalog` with its own test, so 941 reuses it.
6. Document `disputed`: who sets it, whether it applies per fact or per array, how a reader picks among
   conflicting facts (schema `description` and Spec.md).
7. Change report: `category` and `sources` become closed enums; document the snapshot `id`, report
   `before`/`after` and `field` path formats (pattern or description). Add a fixture for patch semantics
   (omitted field inherits; `enabled` vs `default_enabled`).
