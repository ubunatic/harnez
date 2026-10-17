# 797 — respect spec: usage cache freshness policy

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Spec

---

## Problem

Usage cache freshness policy is partly hardcoded in Go and duplicates the
embedded collector cadence. This lets the cadence and the ages that control
recollection, availability, and display drift apart without a spec change.

## Candidates

| Location | Value | Target spec file and key |
|---|---|---|
| `internal/usage/statecache.go:31` | 900 seconds (already `collector_cadence: 15m`) | `spec/usage.yaml: collector_cadence` |
| `internal/usage/statecache.go:45` | Cache staleness is 2 × collector cadence | `spec/usage.yaml: cache_staleness_multiplier` |
| `internal/usage/statecache.go:145` | Display staleness is 7 days | `spec/usage.yaml: display_staleness` |

The availability maximum age at `internal/usage/statecache.go:49` aliases the
cache staleness policy and should continue to derive from that loaded value,
not become a second independent setting.

## What stays in code

`StateDir` path composition and freshness comparison mechanics stay in Go;
they implement filesystem and time operations rather than policy values.

## Acceptance criteria

- Declare all three policy keys in `spec/usage.yaml` and its JSON schema.
- Code reads each value from the embedded usage spec, with no mirrored Go
  fallback.
- Add a test that loads the embedded spec and asserts these keys exist and are
  consumed.
