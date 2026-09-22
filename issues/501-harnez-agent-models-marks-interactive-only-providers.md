# 501 — harnez agent models marks interactive-only providers

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: UX
**Related**: 500 (agy batch driver), `cmd/harnez/agent.go` (`agentDriver`), `internal/subagent/driver.go` (`UnsupportedDriver`), `spec/agent.yaml`

## /goal

`harnez agent models` (text and `--json`) shows, per model, whether `harnez agent start|resume`
can run it in batch mode, so a host never dispatches to a model that only supports `chat`.

## Problem

Before 500, `agy:flash` was listed as a normal model although `agentDriver` returned
`UnsupportedDriver` for `agy`; the gap surfaced only as a runtime error on `start`.
Today all listed providers have batch drivers, so this is a guard against the next provider
added to `spec/agent.yaml` before its driver.

## Notes

- Derive the capability from the driver selection (e.g. "is it `UnsupportedDriver`"), not from a
  second hand-maintained list.
- Suggested text marker: `(interactive only)` after the spec, like the existing `(default)`.
- Test: a fake provider in a test spec is listed with the marker; existing providers are not.
