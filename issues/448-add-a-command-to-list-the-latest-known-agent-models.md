# 448 — Add a command to list the latest known agent models

**Status**: Closed — implemented in cf9e94c
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Usability

---

## 1. Problem & Motivation

`harnez agent start` accepts provider/model/tier identifiers, but there is no
supported command to discover which model aliases and tiers are currently
configured. Documentation can become stale, and users may try unsupported
providers or model names.

## 2. /goal

Provide a read-only `harnez` command that lists the latest known agent models,
providers, aliases, and supported tiers in the format accepted by
`harnez agent start`, with enough source/version context to identify stale
entries.

## 3. Scope and Constraints

- Derive the listing from the same maintained registry/configuration used by
  agent dispatch where practical.
- Clearly distinguish known/configured models from currently available or
  authenticated models; do not imply live availability without checking it.
- Keep the command read-only and suitable for terminal use and scripts.
- When `harnez agent` receives an unknown model or tier, fall back to the
  nearest supported lower model/tier instead of failing outright when a safe
  fallback exists.
- Emit a clear warning for every fallback, including the requested value, the
  selected fallback, and a list of known models/tiers.
- Do not add model execution, health checks, or provider-specific feature
  configuration to this ticket.

## 4. Acceptance Criteria

- A documented `harnez` command lists all latest known provider/model/tier
  combinations and their display names or aliases.
- Output shows the exact identifier users can pass to `harnez agent start`.
- The command has stable human-readable output and a machine-readable form if
  the CLI's existing conventions support one.
- Invalid model and tier inputs produce a warning and select the nearest lower
  supported fallback when one exists; the warning includes known valid choices.
- Fallback behavior is deterministic and covered by tests, including requests
  with no safe fallback remaining.
- Tests cover registry loading and representative output, including unsupported
  or stale entries being labeled rather than silently omitted.
