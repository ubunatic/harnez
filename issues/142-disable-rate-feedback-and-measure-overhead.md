# 142 — Allow disabling `harnez rate` feedback and measure its token overhead

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[117-harnez-rate-command]], [[120-harnez-stats-analytical-reporting]], [[122-agent-instruction-tool-feedback-protocol]]

---

## 1. Problem & Motivation

The mandatory `harnez rate` feedback loop adds an extra agent tool call after
each internal action. Users need a supported way to disable that feedback when
its observability value does not justify the added token and interaction cost.

The trade-off should be measured from current telemetry data, rather than
assumed, so the default and opt-out guidance are evidence-based.

## 2. Technical Specification / Findings

- `harnez stats --auto` records current-session feedback-call volume, but the
  existing `tool_calls` schema does not store model input or output token
  counts for `harnez rate` calls.
- The measurement design must distinguish command-text overhead, tool-call
  protocol/context overhead, and any agent reasoning/output induced by the
  required feedback instruction.
- Disabling feedback must be explicit, scoped, and discoverable without
  silently disabling unrelated telemetry such as `harnez exec` records.

## 3. Implementation & Verification Plan

- Define a supported configuration or runtime switch that disables only
  `harnez rate` feedback requirements and their instruction injection.
- Preserve existing telemetry behavior by default and verify the opt-out does
  not affect `harnez exec`, `harnez stats`, or unrelated hooks.
- Extend telemetry or reporting as needed to measure the additional token
  cost from real sessions, clearly labeling estimates versus provider-reported
  token counts.
- Compare opt-in and opt-out sessions using the current data model, document
  the result, and add focused tests for configuration, instruction generation,
  and reporting.
