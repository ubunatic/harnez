# 743 — Ship harnez-channel as an approved plugin channel so --channels skips the development warning

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 742 (recipient setup), 740 (channel), docs/AgentMessaging.md

---

/goal Start a Claude session with `claude --channels plugin:harnez-channel@<marketplace>` and no "Loading development channels" confirmation, using a documented approval route, verified live; or stop and report when the route needs an organization allowlist or a user decision.

## 1. Problem & Motivation
`--dangerously-load-development-channels server:harnez-channel` asks for confirmation on every
launch (user, 2026-10-07). `--channels` takes only `plugin:<name>@<marketplace>` entries, with an
allowlist enforced.

## 2. Technical Specification / Findings
Package `harnez channel` as a Claude Code plugin in a harnez marketplace; find out from the
channels docs how a plugin channel gets approved (allowlist, managed settings) for a solo user.

## 3. Implementation & Verification Plan
Canary first (docs/Canary.md), then setup output and AgentMessaging.md; live launch without the warning.
