# 737 — Measure webpage screenshot inspection costs and reduce guidance if warranted

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Performance
**Related**: [Doc-image benchmark (387)](387-benchmark-token-efficiency-of-doc-screenshots-and-images-vs-raw-markdown-text-across-agent-harnesses.md), [Website skill source](../docs/commands/website.md)

---

## 1. Problem & Motivation
Agents building webpages appear to capture and inspect screenshots repeatedly, adding noticeable time and potentially substantial token usage. The owner suspects recent advice encourages this too often; the source of that advice and actual image costs are not yet established.

## 2. Technical Specification / Findings
Determine whether existing telemetry records screenshot captures, image views, elapsed time, and attributable token usage. Report measured costs and frequency by provider/model where available, distinguishing actual usage from estimates and session-level totals that cannot establish image costs. If attribution is unavailable, document the gap rather than claiming screenshots are expensive.

The earlier doc-image benchmark concerns document cards, not repeated webpage inspection; its findings do not establish costs for this workflow. Locate the live guidance that drives screenshot use, including installed/global instructions where relevant.

## 3. Implementation & Verification Plan
/goal Establish the cost of repeated webpage screenshot inspection from available telemetry, and, if material overhead is supported, remove or narrow the advice to rare checks when visual uncertainty warrants them. Finish with evidence, any guidance changes and verification; stop and report if blocked on a user decision or denied permission.

- Before starting, check live code, guidance and recent commits; revalidate this ticket's assumptions.
- Summarize available data, capture/view frequency, token and time overhead, attribution limits, and any missing instrumentation.
- If evidence supports a change, update the canonical guidance and its managed copies to discourage routine repeated screenshots while retaining purposeful visual checks. If evidence is insufficient, record that outcome and any needed follow-up.
- Verify changed guidance and installed copies agree, and report what was measured versus inferred.
