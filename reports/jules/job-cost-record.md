# Ideas run: job-cost-record

## Gap 1: TurnRecord lacks reasoning tokens

**Code:** `internal/subagent/session.go:94`
`		OutputTokens      int    `json:"output_tokens"``

**Proposed Change:** Add `ReasoningTokens int json:"reasoning_tokens,omitempty"` to `TurnRecord`.

**Why:** The `TurnRecord` in `Session` stores input and output tokens but is missing reasoning tokens which are part of modern API billing models (and `TurnResult` already has `ReasoningTokensKnown` and `ReasoningTokens`).

**Smallest Step:** Add the `ReasoningTokens` field to the `TurnRecord` struct in `internal/subagent/session.go`.

## Gap 2: TurnRecord lacks API price and cost

**Code:** `internal/subagent/session.go:96`
`		RatingReason      string `json:"rating_reason,omitempty"``

**Proposed Change:** Add `CostMicros *int64 json:"cost_micros,omitempty"` and `PricingRevision string json:"pricing_revision,omitempty"` to `TurnRecord`.

**Why:** The `TurnRecord` tracks tokens per turn but does not store the exact model pricing revision or calculated API cost at the time of the job. Storing these ensures the historical cost doesn't drift if pricing changes.

**Smallest Step:** Add the cost and pricing fields to the `TurnRecord` struct.

## Gap 3: Session struct missing cost summary

**Code:** `internal/subagent/session.go:80`
`	}`

**Proposed Change:** Add `TotalCostMicros int64 json:"total_cost_micros,omitempty"` to `Session`.

**Why:** While tokens are aggregated (e.g. `TokensCumulative`), there's no aggregate cost metric stored directly on the job session to make high-level reporting cheap and easy.

**Smallest Step:** Add the `TotalCostMicros` field to `Session`.

## Gap 4: TurnQuotaReading lacks price share

**Code:** `internal/usage/turnquota.go:24`
`	}`

**Proposed Change:** Add `SubscriptionPriceShareMicros int64 json:"subscription_price_share_micros,omitempty"` to `TurnQuotaReading`.

**Why:** The goal requires storing "a subscription price share (plan price times quota percent used)" before and after the job. The turn quota snapshots do not currently have fields for this derived metric.

**Smallest Step:** Add the `SubscriptionPriceShareMicros` field to `TurnQuotaReading`.

## Gap 5: compaction_economics missing effort and token breakdown

**Code:** `spec/telemetry.yaml:191`
`  	pricing_revision TEXT NOT NULL DEFAULT '',`

**Proposed Change:** Add `effort TEXT` and individual token counts (like `input_tokens INTEGER`, `output_tokens INTEGER`) to the `compaction_economics` table schema.

**Why:** The target states "every job stores tokens in/out/cache-read/cache-write, model, effort, quota before and after... in the database". `compaction_economics` tracks pricing and cost but doesn't store the raw token values or the job's `effort` parameter.

**Smallest Step:** Update the `compaction_economics` CREATE TABLE statement in `spec/telemetry.yaml` and the `insert_compaction_economics` statement.
