# 929 — Preserve remote load sample freshness across SSH polling

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: [920](920-usage-server-collects-remote-load-over-ssh-slow-polling-without-viewers.md), [Epic 921](921-epic-harnez-server-as-monitor-for-local-and-remote-ai-hosts.md), 919

---

## 1. Problem & Motivation
/goal Make remote polling obtain a real CPU/RAM/swap/GPU sample and retain its observation time through the cache and UI. Exit when a production CLI round-trip test proves non-nil load and truthful freshness; stop and report if choosing the wire protocol requires an owner decision.

The SSH command is harnez usage --json, but the current CLI reads the latest stored quota summary, and RenderJSON only marshals it. CollectRegistered and CollectRegisteredCatchup construct summaries with timestamp and agents, without Load. A current remote therefore returns no load and the poller never refreshes the local load cache, even after 920 M3 fixes SSH connectivity.

## 2. Technical Specification / Findings
- internal/usage/remote.go:65 invokes the quota CLI; collectRemoteLoadSnapshotRaw returns summary.Load without obtaining a sample or its timestamp.
- cmd/harnez/usage_cmd.go:185-198 reads via LoadLocalUsageWithStatus; internal/usage/readstore.go:35-47 returns a stored summary; internal/usage/registry.go:441,470 construct quota-only summaries; internal/usage/usage.go:151-155 only marshals them.
- internal/usage/collector_remote_load.go treats a nil load as failure. Successful legacy/stored payloads with a load also lose their original timestamp: remote_load_cache.go:67 sets UpdatedAt to local receipt time. Re-fetching an unchanged sample can consequently display it as fresh indefinitely.
- Separate from 920 M3 shared SSH flags and transition logging. Its fake SSH fixtures can pass while the actual remote CLI returns no load.

## 3. Implementation & Verification Plan
- Choose an explicit SSH load-sampling command or a server-owned load snapshot response, retaining SSH as the transport per 920 scope.
- Carry sample observation time separately from transport receipt time; do not mark an old payload fresh on each successful SSH call.
- Integration test through the actual remote-side CLI with isolated state, asserting CPU/RAM/swap/GPU fields rather than a handcrafted SSH JSON fixture.
- Test repeated identical old payloads and stopped remote collection: age must continue increasing and become stale. Verify current-binary remote polling updates a previously stale cache.
