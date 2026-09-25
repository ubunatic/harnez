# 560 — Consolidate Usage Collection into a Reusable Usage System and Go Library

**Status**: In Progress
**Priority**: P1 (High)
**Severity**: Major
**Category**: Architecture
**Related**: [[033-usage-shared-quota-cache]], [[082-agent-usage-collector-daemon]], [[086-offline-degraded-cache-snapshot-masks-live-data]], [[087-generalize-flock-freshness-gate-to-codex-agy]], [[111-per-agent-collector-pipelines-independent-cadence-timeout-and-cancellation]], [[152-move-agent-collector-under-usage-instead-of-top-level]], [[161-collector-remote-control-host-and-prometheus-exposition]], [[208-sqlite-export-format-for-harnez-usage-export]], docs/studies/2026-08-28-usage-collector-daemon-architecture.md, docs/studies/2026-08-30-usage-panel-integration-and-the-agy-collector-cliff.md

---

## /goal

Provide a reusable Usage System and importable Go library so harnez and independent alert, GUI, and TUI applications can read consistent usage data and, when explicitly requested, share one rate-limited collector/controller. Continuous collection remains optional; apps can use persisted snapshots without a service.

## 1. Problem & Motivation

Usage collection is currently implemented inside `internal/usage`, so consumers outside this Go module cannot import the package. Readers such as `harnez usage` and its TUI call the same internal collectors; separate applications must either parse harnez-owned files or implement provider collection themselves. The latter risks duplicate polling against provider endpoints and provider CLI tools.

The repo already has two useful pieces, but they do not form a stable app-facing system:

- `CollectAll` reads one JSON snapshot per provider from `$XDG_STATE_HOME/harnez/agents/usage` (or `~/.local/state/harnez/agents/usage`) and live-collects missing or older-than-30-minute entries. The collector service is a separate opt-in systemd user service, not a requirement for reading usage.
- Each provider also has a short-lived cache at its provider config directory, using a 30-second freshness window. Same-process mutexes serialize check/fetch/write; a bounded `flock` currently protects writes. Since a lock miss does not prevent a live request, two processes seeing a cold/stale entry can still make duplicate requests. AGY additionally has a shared 30-minute backoff after detecting an authentication-required response.

This work consolidates provider collection, freshness/error reporting, persistence, and coordination behind a stable Go API and data contract. It should make duplicate collector startup discoverable and harmless, while retaining useful offline/stale reads when no collector is running.

## 2. Current-State Findings

- `internal/usage/usage.go:86-110` exposes `CollectAll` and `CollectAllLive` only from an `internal` package. `CollectAll` runs Claude, AGY, and Codex paths concurrently and prefers the shared snapshot cache; `CollectAllLive` bypasses that cache for the daemon.
- `internal/usage/statecache.go:26-49, 77-95, 105-135, 149-180` defines the 15-minute service interval, 30-minute recollect age, seven-day display age, XDG state path, per-agent `AgentSnapshot` JSON, atomic rename writes, and preservation of richer data on lossy/offline writes. Its documented single-writer assumption has no inter-process ownership lock.
- `internal/usage/livefetchcache.go:26-44, 77-136` defines provider-local `harnez-quota-cache.json`, a 30-second fetch freshness floor, bounded advisory flock retries, atomic writes, and an in-process per-path mutex. The mutex spans fetches only within one process. The cross-process flock protects writes only; when it is unavailable the collector still fetches and simply skips persistence.
- `internal/usage/claude.go:311-389`, `codex.go:300-383`, and `agy.go:411-490` implement provider-specific cache checks, fetches, success persistence, and stale fallback. AGY's auth cooldown is 30 minutes (`agy.go:117-126`); Codex also checks whether cached windows have expired. `internal/usage/usage.go:48-64` retries a quota failure once after 200 ms, which is a retry policy that the consolidated controller must review rather than blindly multiply.
- `internal/usage/collector.go:26-67` runs an immediate collection then a single ticker loop and writes the shared snapshot files. `cmd/harnez/main.go:498-523` exposes `agent-collector`, its interval, one-shot, and offline flags. The service is optional: `harnez apply --systemd` installs the unit, and the user separately enables it; see `internal/claude/apply.go:429-470, 1452-1461` and `systemd/harnez-agent-collector.service:1-19`.
- `internal/usage/types.go:9-22, 42-84, 244-250` supplies the current normalized quota, token, provider, and summary shapes. `LastRefreshed`, `QuotaFetchError`, and source labels carry some freshness/provenance information, but the persisted `AgentSnapshot` envelope has no explicit schema version or per-field fetch state.
- Historical stores are distinct: full snapshots are JSON files under XDG state (`statecache.go:77-95`); general timelines and quota windows are JSONL under `~/.claude/harnez/usage-history` (`history.go:32-39, 227-310`; `quota_history.go:138-211`). Provider-local short caches remain alongside auth/config data. Preserve these readers during migration.
- `internal/statusline/statusline.go:57-105` renders provider-supplied stdin payloads and does not currently fetch usage. `internal/agymeter/meter.go:38-50, 80-110, 146-171` is a separate opt-in per-child proxy that records observed AGY token/quota events to `~/.harnez/agymeter/usage.jsonl`; it complements polling and must not be confused with, or silently made dependent on, the shared quota controller.
- Related scope already exists: 082 provides the optional periodic snapshot writer; 033/087 provide provider cache gates; 111 concerns per-provider cadence/timeout/cancellation; 161 concerns moving remote-load ownership and optional Prometheus exposition; 152 concerns CLI command placement. This ticket establishes the shared library/controller and app contract, without absorbing those independent UX and remote-load goals.

## 3. Desired Architecture

### 3.1 Public Go package and API

Add an importable package at module path `ubunatic.com/harnez/usage` (outside Go's `internal/` boundary). Keep provider-specific parsing, credentials, and CLI integration in internal packages; adapt those results to public package types. The public package should offer a small stable surface, with context-aware operations and caller-owned lifecycle:

```go
type Client interface {
    Snapshot(ctx context.Context, opts ReadOptions) (Snapshot, error)
    Subscribe(ctx context.Context, opts ReadOptions) (<-chan SnapshotEvent, error)
    Refresh(ctx context.Context, providers ...string) (Snapshot, error)
    Close() error
}

func Open(opts Options) (Client, error)
```

Use concrete exported structs in the real API; the sketch names the required operations, not a demand for these exact signatures. `Open` must default to reading persisted data and connecting to an existing controller. Starting a collector is a separate explicit option (`StartIfAbsent` or a separately named `StartController` operation); a default `Open` must not spawn processes or start a 24/7 service. `Refresh` must use the shared controller when connected, and otherwise follow a documented direct-collect/cache policy that preserves the provider gates. `Subscribe` can stream changed snapshots/events over IPC; callers that only need simple or offline reads should be able to call `Snapshot` and read persisted data without IPC.

### 3.2 Stable snapshot contract

Define a versioned, provider-neutral envelope and JSON representation. At minimum it must carry:

- `schema_version`, a stable provider ID, and the normalized usage payload (token totals, quota windows with absolute reset times, model groups, plan/account fields where available).
- `observed_at` / `fetched_at`, plus freshness/provenance status distinguishing live success, cached success, stale fallback, skipped/not applicable, and fetch error. Do not represent a failed refresh as a newly-fetched successful value.
- A structured error category and retry/backoff-until timestamp where available; omit credentials, access tokens, and provider response bodies.

Keep JSON additive within a schema version. Document field meaning, timestamp conventions (UTC RFC3339), optionality, and compatibility expectations. Existing `AgentUsage` JSON consumers continue to work through a compatibility reader during migration; the public contract must not expose internal source file paths or depend on renderer-specific strings as machine-readable status.

### 3.3 One collector/controller per user state root

Implement one controller that owns scheduling, live fetches, and writes. Discover it under the per-user runtime directory (`$XDG_RUNTIME_DIR/harnez/usage` where available); use a Unix-domain socket for snapshot reads, refresh requests, and change subscription. Use a process-wide lock/lease beside the socket so only the lock holder serves and refreshes. The contender sequence must be: connect and protocol handshake; if absent, attempt the lock; recheck the socket after winning; then start the controller. Clean up socket/metadata on orderly shutdown, and rely on the lock to recover after a crash. Runtime files and socket permissions must be user-only. Set bounded request and shutdown deadlines; reject incompatible protocol versions clearly.

If no usable runtime directory or Unix socket is available on a supported platform, apps must still read the versioned state files. Specify a deliberate platform fallback before implementing one; do not add TCP listeners or a second transport without a demonstrated need. A same-user local IPC client can request refreshes, but must never provide a way to read or return credentials.

### 3.4 Ownership and opt-in lifecycle

- Ordinary library reads and all existing CLI usage reads work when no controller exists. They may use fresh persisted snapshots and an explicit one-shot refresh path subject to the shared fetch gate.
- An application may explicitly request a controller when it needs fresh/subscribed data. Concurrent applications connect to the existing controller rather than starting collectors themselves. A controller started on demand should have a documented foreground owner/idle shutdown rule so it is shared during active use but does not become an accidental permanent service. Do not detach an unowned background process.
- Keep 24/7 collection explicitly opt-in through the existing user service (or its successor). Installing a unit and enabling/running it remain separate actions. Existing `agent-collector --once` and offline behavior remain available during transition.
- Make lifecycle/status discoverable through the library (connected, started-by-this-client, service-owned, unavailable) without conflating systemd's unit state with a live controller connection.

### 3.5 Rate limits, freshness, and failure behavior

The controller is the single scheduler and gate for each provider. Before every live request it must re-read/check the shared provider result while holding the refresh lease, so two processes cannot both decide from the same cold cache and fetch. A refresh request for a provider already in flight joins that operation or receives its result; it does not create another provider call.

Define explicit per-provider minimum request spacing/TTL, request timeout, and failure backoff. Start from current 30-second minimum and provider-specific behavior, but verify against provider semantics and the 15-minute service cadence; do not make routine forced refresh bypass rate-limit protection. Honor `Retry-After` when supplied, use bounded exponential backoff with jitter for retryable failures, and use a circuit breaker/backoff for AGY interactive reauthentication (preserving its existing cooldown). Keep stale last-known-good data available with clear stale/error metadata. Avoid stacked retries: decide whether the current 200 ms duplicate retry in `usage.go:48-64` remains, and bound all retries under the controller policy. Do not let a reader poll provider APIs merely because another app is open.

### 3.6 Persistence and migration

Use the existing XDG state location as the canonical snapshot root for compatibility. Add explicit schema versions and preserve atomic temp-file/rename semantics and `0600` data/lock permissions. The controller is the single snapshot writer; library readers tolerate absent, corrupt, and newer-unsupported files with explicit errors/fallbacks. Migrate current `<agent>.json` snapshots by reading legacy shape and writing the new contract on a successful collection; never delete or rewrite history as part of migration. Continue reading/appending existing `~/.claude/harnez/usage-history/*.jsonl` and `quota-history.jsonl` until a separately planned migration exists. Keep provider-local quota cache compatibility while it is needed for older harnez processes, and establish a retirement/dual-read policy so two cache authorities do not persist indefinitely.

## 4. Application Integration

- Move `cmd/harnez` usage display, JSON output, watch refreshes, and `usage history record` to the public client or shared controller interface. Preserve current CLI output and provider behavior during rollout.
- External alert tools can import the Go library or read the documented versioned state contract without opening provider credentials. GUI/TUI consumers can subscribe for updates and render their own UI.
- Keep status-line invocations cheap and non-blocking. Since provider status-line payloads already include supplied context/quota data, use the Usage System only for a bounded persisted read if that feature is added; do not perform live provider calls in status-line callbacks.
- Keep AGY metering as an independent event source. Define whether its token events are exposed through the public contract later; this ticket only requires that the shared controller neither duplicates nor drops the meter's current JSONL data.
- Do not implement a Control Panel UI, alert rules, Prometheus endpoint, remote collection, or provider sources beyond existing Claude/Codex/AGY in this ticket.

## 5. Implementation Milestones and Acceptance Criteria

### M1 — Public contract and compatible persistence reader

- Add the importable `ubunatic.com/harnez/usage` package, documented API and versioned snapshot/event structs; keep provider internals private.
- Add fixtures for current persisted snapshots and prove the new reader decodes legacy per-agent JSON, handles unknown schema versions, and never exposes auth material.
- Add round-trip and compatibility tests for timestamps, optional quota/token fields, stale/error status, and additive unknown JSON fields.
- Existing `harnez usage --json` shape/output remains compatible unless a separately versioned public response is explicitly selected.

### M2 — Controller lock, discovery, and local IPC

- Add controller ownership using a user-only runtime lock and socket. Race tests with concurrent startup attempts prove exactly one owner/listener and that losers connect after the owner publishes its endpoint.
- IPC supports version handshake, snapshot read, subscription/change notification, and a bounded refresh request; malformed requests, disconnects, owner crash, and stale socket recovery fail safely.
- IPC refresh requests and direct one-shot refresh share the same per-provider fetch gate. Tests prove two independent processes cannot issue duplicate mock provider calls for one cold cache.
- A client can read persisted snapshots without runtime directory/socket availability; no network listener is created.

### M3 — Central collection policy and lifecycle

- Move provider scheduling and writes under controller ownership while reusing existing collectors. Serialize cross-process check→fetch→write, coalesce in-flight refreshes, honor provider spacing/backoff, preserve last-known-good values, and expose per-provider outcomes.
- Default library open/read does not spawn a process. Explicit `StartIfAbsent` starts/attaches to exactly one foreground-owned controller; document and test ownership, concurrent attach behavior, cancellation, and idle shutdown.
- The 24/7 user service remains opt-in and uses the same controller mode. Existing `--once`/`--offline` semantics are retained or replaced with documented equivalents.
- Tests use mock HTTP/command providers and temp XDG roots; no live provider account is needed. Include failure/backoff and stale-result tests for each existing provider and AGY reauth cooldown.

### M4 — Harnez and consumer integration

- Migrate CLI one-shot, watch, JSON, and history recording paths to the shared client/controller without regressing current render/output tests.
- Add a small external-package compile/example test proving a consumer outside `internal/` can import the public package, read a snapshot, and subscribe.
- Verify a second app instance attaches to an already-running controller and that closing the client does not terminate a service-owned controller.
- Document the contract and lifecycle for app authors, including the no-service file-read path and explicit auto-start choice.

### M5 — Migration/retirement and operational verification

- Test upgrade with existing state and provider cache fixtures. Confirm snapshots/history survive version transition, remain readable on rollback for the documented compatibility window, and preserve file permissions.
- Confirm no-controller usage still works; explicit controller startup works; optional systemd service works; stopping foreground owners and service leaves no collector or socket orphan.
- Record which provider-local cache paths remain necessary and the condition for retiring old readers/writers. Do not remove legacy support without a demonstrated upgrade path.

## 6. Risks and Open Questions

- **Runtime portability:** current flock implementation imports `syscall.Flock`; public-library consumers may use non-Linux platforms. Decide supported OSes and the lock/socket abstraction before making `Open` promise cross-platform controller startup. File-only reads should remain portable wherever possible.
- **Foreground sharing lifetime:** choose an idle grace period and owner-exit behavior that lets concurrently launched apps share one controller without leaving it running forever. If robust process supervision cannot be implemented safely in-process, prefer explicit service start over detached spawning.
- **Forced refresh semantics:** determine whether to omit force refresh or expose a request that still observes provider cooldown and `Retry-After`. It must not become a bypass for account protection.
- **Schema scope:** decide whether the public data contract reuses public aliases of current `AgentUsage` or introduces independent versioned structs. Prefer independent contract types if internal fields/source labels are likely to change; avoid maintaining two unsynchronized schemas.
- **Old cache coexistence:** snapshot cache, provider-local short cache, quota history, general history, and AGY meter records have different roles. Document source of truth and retirement conditions to avoid migration data loss or repeated provider fetches during mixed-version operation.
- **Two candidate features overlap only partially:** 161's remote-load migration and optional Prometheus exposition are explicitly out of scope; coordinate controller ownership/lifecycle interfaces with it if either is implemented first. 152's CLI move is also independent; avoid coupling the public API to the top-level command name.
- The API and IPC are durable compatibility surfaces. Keep the first contract small, version it before external adoption, and avoid promising arbitrary provider refresh/plugins in this milestone.

## 7. References

- `internal/usage/usage.go:48-64, 86-166` — retry behavior, cache-first collection, current collectors.
- `internal/usage/statecache.go:26-49, 77-180, 190-260` — XDG snapshots, freshness, atomic writes, lossy/offline protection, read fallback.
- `internal/usage/livefetchcache.go:26-136` — per-provider cache, flock and in-process mutex scope.
- `internal/usage/claude.go:311-389`, `internal/usage/codex.go:300-383`, `internal/usage/agy.go:117-126, 411-490` — provider-specific fetch/cache/backoff behavior.
- `internal/usage/collector.go:26-67`, `cmd/harnez/main.go:219-340, 498-523` — current daemon loop and CLI integration.
- `internal/usage/types.go:9-84, 244-250`, `internal/usage/history.go:32-39, 227-310`, `internal/usage/quota_history.go:138-211` — current data types and persistence.
- `internal/statusline/statusline.go:57-105`, `internal/agymeter/meter.go:38-50, 80-110, 146-171` — status-line and separate AGY metering paths.
- `internal/claude/apply.go:429-470, 1452-1461`, `systemd/harnez-agent-collector.service:1-19` — optional service install/run lifecycle.
- `issues/033-shared-quota-cache.md` (archived path/title may differ), archived issue 087, issues 082, 111, 152, 161, 208, and studies listed in Related — prior decisions and adjacent scope.
