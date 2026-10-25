# 933 — Usage discovery regression: collector marks all agents uninstalled

**Status**: Open
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

## 3. Implementation & Verification Plan
Compare shell agent-collector --once with harnez-agent-collector.service; permit
only collector live-state changes. Diagnose and report before source edits.
Acceptance: agents installed in the shell remain discovered in service snapshots,
quota data remains visible, and a targeted regression check identifies the culprit.
