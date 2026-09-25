# 596 — agent start: preflight Codex auth before starting an agent

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 595, 592

---

## 1. Problem & Motivation
On 2026-09-26 two reviewer starts failed with Codex 401 Unauthorized while `codex login status` said "Logged in". Each start burned ~40s and showed a misleading error (595).

## 2. Technical Specification / Findings
`codex login status` only checks local credentials, not whether the server accepts them. A real preflight needs a cheap authenticated request.

## 3. Implementation & Verification Plan
Canary first: find the cheapest call that returns 401 on a rejected token. Then check once before `agent start`/`resume` for Codex (cache the result briefly) and fail with "Codex login rejected; run `codex login`". Test with a stubbed failing check.
