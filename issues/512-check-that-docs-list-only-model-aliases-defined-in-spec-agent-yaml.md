# 512 — Check that docs list only model aliases defined in spec/agent.yaml

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: `spec/agent.yaml`, `docs/HarnezAgentArchitecture.md` §3, `docs/Models.md`, `docs/Spec.md`

## Problem

Until 697216d, `docs/HarnezAgentArchitecture.md` §3 listed the aliases `agy:flash`,
`agy:flash:med` and `local:lmcoder` and wrong Claude flags, none of which matched
`spec/agent.yaml`. Nothing checked that the doc matched the spec, and the drift was only
noticed by chance during an evergreen pass.

## /goal

A test or `make check` step fails when a doc under `docs/` (outside `studies/`, `feedback/`
and other dated records) names a `provider:model[:tier]` alias for a known provider (codex,
claude, agy) that `subagent.ResolveModel` rejects.

## Done when

- The check runs in `make check` and passes on the current docs.
- Adding a bogus alias such as `agy:flash` to an evergreen doc makes it fail with the
  file, line and alias.
