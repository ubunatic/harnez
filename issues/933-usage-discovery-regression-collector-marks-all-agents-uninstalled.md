# 933 — Usage discovery regression: collector marks all agents uninstalled

**Status**: Closed — b0356c2a: store catch-up restores agent metadata (Installed). Live after install and server restart: All Usage shows Claude, Codex and Gemini rows again. Full run's only failure is 932's remote one-shot test, unrelated
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: [Epic 921](921-epic-harnez-server-as-monitor-for-local-and-remote-ai-hosts.md)

---

## 1. Problem & Motivation
Every usage view reports no agent usage detected. The service snapshot contains
Claude/Codex/AGY quota data but marks every agent installed=false; worked around
13:05 on 2026-10-25.

/goal Restore agent discovery in every usage view with a verified regression fix;
stop and report after diagnosis and a fix plan until the host authorizes code edits,
or when blocked on an owner decision or unavailable environment evidence.

## 2. Technical Specification / Findings
Investigate installed assignment, service sandbox and PATH versus shell execution,
and commits ba63980d, b0c32ecd, 612c2927, 6d07b130, f993083b, 7f4d55cb,
f0031376, and 20213dac. Bisect using an executable regression predicate.

### Diagnosis (2026-10-25)

- Initial service summary: all three providers installed=false/authenticated=false,
  with quota windows retained. Shell `harnez agent-collector --once` succeeded and
  the latest summary then had installed=true/authenticated=true for all three.
- A transient one-shot collector with the unit's PrivateTmp=yes,
  ProtectSystem=strict, ProtectHome=read-only and expanded ReadWritePaths also
  succeeded (8.623s); its summary retained true for all three providers. The
  transient unit was collected automatically; the existing service was not changed.
- The running service has HOME=/home/uwe and the user-manager PATH. Its mount
  namespace exposes .claude, .codex and .gemini. CollectClaude/CollectCodex and
  collectAGYWithHome set Installed from directory existence, not executable PATH.
- `statecache.go:414` creates AgentUsage with only AgentID/LastRefreshed.
  applyStoredWindows deliberately preserves its incoming metadata, so Installed,
  Authenticated, Name and other metadata stay at zero values. The snapshot writer
  also serializes this reduced object into the compatibility JSON mirrors.
- Commit 7f4d55cb rewrote `usagestore.CurrentFor` row selection and added memoized
  provider availability in `statecache.go`; CurrentFor still returns normalized
  quota windows only. `agentSnapshotFromStore` and `ProviderSnapshotFromStoreAt`
  build AgentUsage values from those rows without restoring metadata. The
  metadata-dropping snapshot path therefore runs in the 924-era code, while its
  constructor first appeared in eb26fbae.
- `CollectRegisteredCatchup` seeds lastGood from this SQLite reconstruction;
  CollectCatchup skips live collection for fresh entries and persists the reduced
  objects as a complete summary. This explains why daemon catch-up can hide agents
  while a one-shot live collection restores them. Catch-up was added by 9e247bf8
  (2026-10-23); the exact event triggering today's occurrence was not captured.
- `git bisect run` in an isolated bare shared clone identified eb26fbae
  (2026-09-30) as the first metadata-dropping constructor. Its function text is
  identical in all eight supplied suspect revisions (SHA256 prefix 01a2883ed5b0).
  The executable probe extracts the historical production constructor and runs it
  with a nonempty quota row and a stub preserving applyStoredWindows' metadata
  contract; installed=false fails. Earlier revisions without that constructor
  are classified good by construction. This is an isolated constructor regression
  probe, not a full historical end-to-end test or proof of today's trigger.
  Probe and bisect log: /tmp/harnez-933-bisect-yylhgekq/{probe.py,repo.git}.

## 3. Implementation & Verification Plan
Compare shell agent-collector --once with harnez-agent-collector.service; permit
only collector live-state changes. Diagnose and report before source edits.
Acceptance: agents installed in the shell remain discovered in service snapshots,
quota data remains visible, and a targeted regression check identifies the culprit.

### Fix

Preserve full agent metadata when combining normalized quota windows with agent
snapshots, and retain it when writing mirrors and seeding catch-up. Cover both
ReadAgentSnapshot and ProviderSnapshotFromStoreAt, whose reconstruction has the
same zero-metadata issue. Do not infer authentication solely from quota rows.
Use the latest complete summary as metadata when repairing already-reduced mirrors,
or perform local discovery when no trustworthy metadata survives. Add regression
coverage for live collection -> persistence -> cached read -> fresh catch-up ->
stored summary, including installed/name/authentication/account/model and quota
preservation. The regression test exercises collector persistence, server catch-up
and the persisted read path. Run the touched package tests, `go vet ./...`,
`make lint`, and `make install`; the host restarts the unit and verifies the UI.
