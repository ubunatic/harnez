# 799 — respect spec: RAMP assessment rules and labels

**Status**: Closed — Moved all 34 RAMP vocabulary and description values into the validated embedded spec; output goldens unchanged and required checks passed (ee796a7a)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Spec

---

## Problem

The RAMP assessment's versioned classification vocabulary, levels, categories,
confidence values, artifact statuses, and user-facing descriptions are defined
as Go constants and switch results. This is application policy and content,
so the current `ramp-v1` rule set cannot be inspected or changed through the
embedded YAML spec.

## Candidates

| Location | Value | Target spec file and key |
|---|---|---|
| `internal/assess/ramp.go:16` | Rule set version `ramp-v1` | `spec/ramp.yaml: version` |
| `internal/assess/ramp.go:24` | Levels `L1`–`L4` and their descriptions | `spec/ramp.yaml: levels` |
| `internal/assess/ramp.go:35` | Nine category IDs grouped across levels | `spec/ramp.yaml: categories` |
| `internal/assess/ramp.go:54` | Confidence labels `high`, `medium`, `low`, `unknown` | `spec/ramp.yaml: confidence` |
| `internal/assess/ramp.go:66` | Artifact statuses `committed`, `uncommitted`, `projected` | `spec/ramp.yaml: artifact_statuses` |
| `internal/assess/ramp.go:97` | Four level titles and nine category descriptions | `spec/ramp.yaml: descriptions` |

This is 34 candidate values: one version, four levels, nine categories, four
confidence labels, three statuses, four level titles, and nine category
descriptions.

## What stays in code

Level ranking, file inspection, sorting, and classification mechanics stay in
Go. They compute results from repository evidence and do not define the
classification vocabulary or its labels.

## Acceptance criteria

- Add `spec/ramp.yaml` and a matching JSON schema for the rule set vocabulary
  and descriptions.
- Code reads those values from the embedded spec, with no mirrored fallback
  constants or description switches.
- Add a test that loads the embedded spec and asserts its keys exist and are
  consumed by the RAMP assessment.

## Delivery

- Implemented in `ee796a7a`: all 34 vocabulary/title/description values now
  live in embedded `spec/ramp.yaml`, validated against
  `spec/schemas/ramp.schema.json`. Logical Go identifiers read spec values;
  category levels and description lookups also consume the spec directly.
- Loading rejects unknown category level references, duplicate vocabulary IDs,
  missing descriptions, malformed YAML, unknown fields, and invalid types.
  Ranking, inspection, sorting, and classification mechanics remain in Go.
- Tests were added first: embedded-key/consumer coverage and negative validation
  cases; 12 goldens captured before implementation prove byte-for-byte text,
  init-preview, and JSON compatibility across baseline, complete, incoherent,
  and non-Git profiles.
- Passed `make lint` (warnings only),
  `go test ./internal/assess/ ./cmd/harnez/`, `make test-short`,
  `git diff --check`, and `make install`. No open implementation problems.
