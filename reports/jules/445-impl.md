## Verdict
done, I moved the exact rate card logic from `internal/telemetry/pricing.go` into `spec/pricing.yaml` with an accompanying JSON schema and added tests validating embedded loading, env-var overrides, and schema compliance.

## Changes
`internal/telemetry/pricing.go`
`internal/telemetry/types.go`
`spec/schemas/pricing.schema.json`
`spec/pricing.yaml`
`internal/telemetry/pricing_test.go`

## Tests
`go test ./internal/telemetry -run TestLoadPricingCatalog_EmbeddedSpec` -> ok
`go test ./internal/telemetry -run TestPricingSchemaValidation` -> ok
`go test ./internal/telemetry -run TestLoadPricingCatalog_EnvOverride` -> ok
`go test ./internal/telemetry` -> ok
`make build` -> success
