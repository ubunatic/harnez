# 142 — Allow disabling `harnez rate` feedback and measure its token overhead

**Status**: Closed — resolved
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

## Resolution Note

### Opt-out mechanism

Primary: a new `feedback.disable_rate_protocol: bool` key in `config.yaml`
(`internal/claude.FeedbackConfig`), default `false`. Secondary: the
`HARNEZ_DISABLE_RATE_FEEDBACK` env var (any value other than `""`, `0`,
`false`, `no`, `off` counts as truthy), for a quick local override without
editing `config.yaml`. Either source being true disables it —
`claude.RateFeedbackDisabled(cfg, getenv)` is the single decision point both
`ApplyAll` and `DiffAll` consult.

Scope: the two `MDSection`/`Command` config entries that constitute the Tool
Feedback Protocol instruction (the `agents_md.global.sections` "Tool Feedback
Protocol" entry and the `tool-feedback-protocol` skill) are now marked
`rate_feedback: true` in `config.yaml`. That's the only thing the toggle
touches — it's a config-entry marker, not a name-matching heuristic in Go
code, so any future gated instruction can opt in the same way.

`harnez apply` behavior when disabled: rather than merely skipping these
entries (which would leave a previously-installed instruction stale on the
next apply), it actively removes them via `markdown.Clean` (sections) and
`os.Remove` (skill files), for every configured target (`~/.claude/CLAUDE.md`,
the Prime Agent `AGENTS.md` mirror, and every skills root). `harnez diff`
reports drift symmetrically: while disabled, "drift" means the gated section
is still present (via `markdown.ContainsSection`), not a content mismatch.
`harnez clean` (full uninstall) is unaffected — it always removes every
managed section/skill regardless of this flag, since its purpose already
implies removing everything harnez installed.

Deliberately out of scope, per the ticket's own framing: `harnez rate`,
`harnez exec`, and `harnez stats` are untouched by the flag — none of their
command implementations consult `cfg.Feedback` or the env var at all, so the
separation is structural, not just tested. See
`TestRateExecStats_UnaffectedByRateFeedbackDisableEnv`
(`cmd/harnez/feedback_toggle_test.go`) for a concrete round-trip proof (rate
+ exec write rows, stats reports them, all with the env var set).

### Overhead measurement

Extended `tool_calls` telemetry rather than adding a schema migration:
`cmd/harnez/rate.go`'s `insertRateRow` now populates the pre-existing but
previously-unused `RawBytes` column with the actual byte length of the rate
call's own argument payload (`rateCallPayloadBytes`: tool_name + score +
quoted description + optional ticket_id) — real, measured data, not an
estimate. Since `call_type="internal"` is written only by `harnez rate`
(confirmed by reading every other write path), it already uniquely
identifies rate-feedback rows with no new column needed.

`internal/telemetry.RateCallOverhead(f Filter)` aggregates those rows
(forcing `f.CallType = "internal"` regardless of what the caller passed) into
count/total-bytes/avg-bytes. `internal/telemetry.EstimateTokens(bytes) int64`
converts bytes to an **estimated** token count via a ~4-bytes-per-token
heuristic, explicitly documented and labeled as an estimate in every place it
surfaces — harnez has no access to the calling model's real tokenizer or
provider-reported usage, so a real token count was not achievable in this
environment; this was flagged in the ticket as an acceptable fallback.

The one-time (per-session, not per-call) instruction-text cost is measured
separately: `internal/claude.ToolFeedbackProtocolBytes(cfg)` sums the byte
length of every `rate_feedback: true`-marked section/skill's content — this
is genuinely different in kind from the per-call cost (it's injected once
into the system prompt, not repeated per `harnez rate` invocation), so the
two are reported side by side rather than merged into one number.

`harnez stats --overhead` (and `--json`) surfaces both: a `rate_feedback_overhead` report with `calls`, `total_call_bytes`, `avg_call_bytes` (measured),
`instruction_bytes` (measured), and `estimated_call_tokens` /
`estimated_instruction_tokens` (heuristic, labeled `estimate_method:
"ESTIMATE: ~4 bytes/token heuristic, not provider-reported"`). It composes
with the command's existing filters (`--tool`, `--agent`, `--ticket`,
`--auto`).

### Finding (evidence-based, per the ticket's ask)

A full live A/B session capture (identical session run twice, once with the
protocol enabled and once disabled, diffing real provider token usage) was
not practical in this environment — no access to the calling agent's
provider-reported token counts from inside `harnez` itself. Instead, this is
a documented estimate from real instruction text and real measured call
data, run against this repo's own accumulated telemetry
(`~/go/bin/harnez stats --overhead`, 2026-09-02, 499 historical rate calls):

```
harnez rate feedback overhead (issue 142):
  calls: 499, total call bytes: 0, avg call bytes: 0.0 (measured)
  instruction text: 2014 bytes, injected once per session (not per call)
  ESTIMATE: ~4 bytes/token heuristic, not provider-reported: ~0 call tokens + ~503 one-time instruction tokens
```

`total call bytes: 0` for the historical calls is itself a finding, not a
bug: those 499 rows predate this ticket's `RawBytes` population, so they
carry the table's pre-existing default of 0 — only `harnez rate` calls made
after this change carry a real payload-byte figure. Verified live after
`make install`:

```
$ harnez rate TestTool 5 "overhead measurement sanity check" harnez/142-...
$ harnez stats --tool TestTool --overhead
  calls: 1, total call bytes: 105, avg call bytes: 105.0 (measured)
  instruction text: 2014 bytes, injected once per session (not per call)
  ESTIMATE: ...: ~26 call tokens + ~503 one-time instruction tokens
```

Interpretation: the one-time Tool Feedback Protocol instruction text
(the global CLAUDE.md section + the `tool-feedback-protocol` skill, both
loaded once per session) is ~2 KB, an estimated ~500 tokens — a fixed cost
independent of session length. Each `harnez rate` call itself is small
(~100 measured bytes of argument payload, ~25 estimated tokens), but per
issue 181's own narrowing (agents rate only failed/unexpected-outcome calls,
not every routine call), the per-call marginal cost was already reduced
before this ticket; the fixed ~500-token instruction cost is the dominant,
session-independent overhead this ticket's opt-out actually removes. A
session that decides the observability value doesn't justify a persistent
~500-token system-prompt tax now has a supported way to remove it via
`feedback.disable_rate_protocol` or `HARNEZ_DISABLE_RATE_FEEDBACK=1`.

### Tests

- `internal/claude/toolfeedback_disable_test.go`:
  `TestRateFeedbackDisabled_ConfigAndEnv` (config/env truthiness matrix) and
  `TestApplyOmitsToolFeedbackProtocolWhenDisabled` (enable → apply → disable →
  re-apply removes the section/skill, unrelated sections untouched, `diff`
  reports clean).
- `cmd/harnez/feedback_toggle_test.go`:
  `TestRateExecStats_UnaffectedByRateFeedbackDisableEnv` (acceptance
  criterion 3 — rate/exec/stats all keep working with the env var set).
- `cmd/harnez/rate_test.go`: `TestRunRate_RecordsCallPayloadBytes` (RawBytes
  populated and proportional to payload size).
- `internal/telemetry/rate_overhead_test.go`: `RateCallOverhead` isolates
  `call_type="internal"` rows and overrides a caller-supplied `CallType`
  filter; `EstimateTokens` heuristic table.
- `cmd/harnez/stats_test.go`: `TestRunStatsOverhead_MatchesHandComputedFixture`
  (table + JSON, hand-computed fixture) and
  `TestRunStatsWithoutOverheadFlag_OmitsOverheadField` (opt-in, no shape
  change for existing callers).

`go test ./...` passes. `go vet ./...` clean.
