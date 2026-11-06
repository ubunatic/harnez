# 1176 — usage --json: contract test for the fields taia-models reads

**Status**: Closed — Done: cmd/harnez/usage_json_contract_test.go asserts every usage --json field taia-models reads, through usage.RenderJSON; fails on a renamed field
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Test
**Related**: taia-plugins `models/harnez.go` (codeberg.org/harnez/taia-plugins), taia#113, 942; study docs/studies/2026-11-06-jules-backlog-cleanup.md

---

/goal A test in harnez fails when `harnez usage --json` drops or renames a field that taia-models reads; or stop and report when blocked on an owner decision.

## Problem

taia's `taia-models` plugin shells out to `harnez usage --json --offline` and decodes
(`taia-plugins/models/harnez.go:17-40`, read 2026-11-06):

- top level: the agents list (`.agents[]`);
- per agent: `agent_id`, `installed`, `authenticated`, `plan_tier`, `stale`, `last_refreshed`,
  `session`, `weekly`;
- per window (`session`, `weekly`): `remaining_percent`, `used_percent`, `reset_at`.

No harnez test asserts these names or types (search 2026-11-06: only
`internal/usage/quota_history_test.go:50` checks `remaining_percent`, on a different record). A
harnez refactor can break taia's model picker silently.

## Want

- One test in the package that renders `usage --json` (find it from `cmd/harnez` usage command) that
  builds the JSON document from a fixed snapshot with one agent carrying both windows, decodes it
  into `map[string]any`, and asserts each path above exists with the JSON type taia expects
  (string, bool, number or null, RFC 3339 string).
- A comment in the test naming taia-models as the consumer, so a rename is a conscious decision.

## Done when

- The test passes on main and fails when any listed field is renamed (show one failing run).

## Jules runs

| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| implement | jules-implement-1176 | 7274565587309179823 | github #133 (closed, integrated by host) | done: test taken, switched to usage.RenderJSON (the real --json path, with staleness fill-in); host ran it green and saw it fail on renamed used_percent and plan_tier |
