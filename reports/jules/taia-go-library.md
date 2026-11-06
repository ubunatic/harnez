# Ideas for `pkg/` Go Library

This report proposes 5 seams to export a Go package (e.g. `pkg/harnez` or similar) to allow other Go programs to consume quota info and model catalog info without shelling out to `harnez`.

## 1. Export `ModelCatalog` initialization

**Target:** `internal/subagent/driver.go:138`
`func loadMergedModelCatalog() error {`

**Proposed Change:** Extract `loadMergedModelCatalog` and the `modelcatalog.Catalog` structures into a public package, e.g., `pkg/catalog`. Expose a `Load() (*Catalog, error)` function.
**Why it helps:** External Go programs can directly parse and interrogate the same model catalog rules (e.g. model aliases, tiers, constraints) as `harnez` without relying on CLI text scraping or private internal state.
**Must not import:** `internal/subagent`, CLI flags, or TUI code.
**Smallest next step:** Move `internal/modelcatalog` to `pkg/modelcatalog` and update internal imports to point to the new package path.

## 2. Export `Model` struct and resolution

**Target:** `internal/subagent/driver.go:422`
`func resolveModelWithCatalog(aliases map[string]modelAlias, catalog *modelcatalog.Catalog, spec string) (Model, error) {`

**Proposed Change:** Move `resolveModelWithCatalog` into `pkg/catalog` alongside the `Model` struct.
**Why it helps:** External terminal agents can pass a model string like `claude-3-opus` and resolve it using Harnez's standard rules to get the canonical provider, name, and tier.
**Must not import:** `internal/subagent/driver` (execution implementations).
**Smallest next step:** Move the `Model` struct (`internal/subagent/driver.go:76`) and `resolveModelWithCatalog` into `pkg/catalog`.

## 3. Export Data Structures for Usage and Quota

**Target:** `internal/usage/types.go:243`
`type UsageSummary struct {`

**Proposed Change:** Move `UsageSummary`, `AgentUsage`, and related usage structures (e.g. `QuotaWindow`) to a public `pkg/usage` package.
**Why it helps:** External Go programs can strongly type the JSON schema or data structs without copy-pasting the struct definitions into their own project, keeping them in sync.
**Must not import:** TUI/rendering logic or direct `telemetry`/`usagestore` dependencies.
**Smallest next step:** Create `pkg/usage` and move `internal/usage/types.go` definitions there.

## 4. Export `CollectAll` functionality

**Target:** `internal/usage/usage.go:84`
`func CollectAll(ctx context.Context, homeDir string, client *http.Client) UsageSummary {`

**Proposed Change:** Expose `CollectAll` (or a similar non-rendering collection function) in `pkg/usage`.
**Why it helps:** An external Go agent can collect all current quota window statuses programmatically in-process (e.g., via background goroutines) without incurring the OS overhead of shelling out to `harnez usage --json` on every check.
**Must not import:** TUI rendering (e.g. `internal/usage/RenderText`).
**Smallest next step:** Refactor `internal/usage/usage.go` to have `CollectAll` return the `pkg/usage.UsageSummary` directly and expose it publicly.

## 5. Export `CaptureTurnQuota` for single provider fast-paths

**Target:** `internal/usage/turnquota.go:43`
`func CaptureTurnQuota(ctx context.Context, provider string, force bool) TurnQuotaReading {`

**Proposed Change:** Move `CaptureTurnQuota` into `pkg/usage` (or similar) to allow fetching just one provider's quotas at turn-boundaries.
**Why it helps:** External agents acting on behalf of a user need to quickly fetch their specific provider quota (e.g. `claude`) immediately after a turn without pulling full state for everything else, making it much faster than a generic CLI call.
**Must not import:** Full `CollectAll` machinery if possible, TUI, telemetry loops.
**Smallest next step:** Move `TurnQuotaReading` and `CaptureTurnQuota` into `pkg/usage`.
