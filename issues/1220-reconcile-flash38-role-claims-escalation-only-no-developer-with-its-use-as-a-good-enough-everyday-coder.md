# 1220 — reconcile flash38 role claims (escalation-only, no developer) with its use as a good enough everyday coder

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agent Guidance
**Related**: 604 (closed: flash38 escalation-only), 711 (flash38/default-spec mismatches), 1002 (agy developer turns on flash38:low), docs/Models.md, spec/agent.yaml

---

## 1. Problem & Motivation

The owner's view is that agy:flash38 is a good enough coder and everyday model. Repo guidance says it is not a developer, and that claim is not measured:

- `spec/agent.yaml:258` gives `agy:flash38` `roles: reviewer, advisor`. `spec/agent.yaml:262` says "use flash38:med for routing and preprocessing".
- `docs/Models.md:195` says "flash37/flash38 are frontier-grade, not mechanical workers". The same table (`docs/Models.md:186`) rates agy:flash38 as ok on Go/TUI/tools and TB2.1 89%.
- Ticket 604 (closed) made flash38 escalation-only and removed the agy developer roles after a cost complaint. It kept COST 4 because there was "no measured agy cost data".

The repo also contradicts itself, and live use contradicts the spec:

- `docs/data/models-001.txt:4` and `docs/data/models-002.txt:4` list `developer` for `agy:flash38:low`.
- `docs/studies/2026-10-31-session-retro-plain-modes-time-overrides-lock-hygiene.md` records `agy:flash38:low` dispatched as mechanical developer (dev-1027, dev-1056, dev-1065).
- Ticket 1002 ran `harnez agent --role developer --model agy:flash38:low` turns.

Agents that read the spec will avoid flash38 for coding, while the owner uses it for everyday coding.

## 2. Technical Specification / Findings

- Verified: the spec roles, the Models.md claims, and the 604 closure are as quoted above (checked 2026-11-12).
- Verified: 711 noted "two flash38/default-spec mismatches remain". The test it names, `TestFlash38EscalationGuidanceAndLeanSprintDeveloperPreference`, no longer exists under that name in `*_test.go`, so the current pinning test is unidentified.
- Unverified: whether flash38 developer output is good enough. The owner's judgement is the only evidence for "good enough". No pass rate or review-reject rate exists in the repo.
- Unmeasured: the flash38 cost. The COST 4 figure is an estimate (604), and the flash38 quota pool and cost are not measured in tokens-to-cost terms.
- Decision needed from the owner: is flash38 (a) escalation-only, (b) a developer tier for everyday work, or (c) developer only at `:low` with review gates?
- Proposed canary (not run): replay about 10 recent developer tickets on `agy:flash38:low` and `codex:luna:low`. Compare test pass, reviewer rejects and rework. One run is a hypothesis, not a result.

## 3. Implementation & Verification Plan

/goal Get the owner's decision on flash38's role (escalation-only, developer, or developer at `:low` with review). Then make `spec/agent.yaml`, `docs/Models.md` and `docs/data/models-*.txt` agree with it, and confirm `harnez agent models` shows the chosen roles. Run `make test`. Stop and report when the owner must choose, or when a permission is denied.

- Ask the owner for the intended role and record the answer in this ticket.
- Align the spec `roles`/`use` text, the Models.md line 195 claim, and the data files with the decision.
- Change COST only with measured data from the canary. Otherwise keep COST 4 and mark it unmeasured.
- Verify: `harnez agent models` matches the decision, and `make test` passes.
