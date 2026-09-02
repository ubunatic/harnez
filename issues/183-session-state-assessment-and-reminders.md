# 183 — Session State Tracking + Proactive Gap Reminders for Coding-Env Callers

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [issues/181](181-narrow-harnez-rate-to-failure-cases.md) (rate policy), [issues/178](178-distill-smart-mode-error-pattern-preservation.md) (distill), [issues/180](180-watch-and-review-compact-commands.md) (compact watching)

---

## 1. Problem & Motivation

When a coding environment (Claude Code, Codex, etc.) invokes harnez in any way (`distill`, `rate`,
`find`, `usage`, ...), harnez has no memory of the calling session's history of harnez usage. This
means harnez can't notice patterns like "this session has distilled 20 outputs and rated none of
them" or "this session has never used `harnez find` despite repeatedly grepping `issues/`
manually" — and can't nudge the agent toward better usage of its own features.

## 2. Technical Specification / Findings

Introduce a lightweight per-session state store (keyed by session id, consistent with existing
env-inference conventions — see prior harnez decision to use directory + session_id as the only
reliable signals, not per-ticket branches) that tracks, at minimum:

- Count and timestamps of harnez subcommand invocations this session (`distill`, `rate`, `find`,
  `usage`, etc.).
- Time/call-count since the last `harnez rate` call, scoped to the narrowed failure-only policy
  from issue 181 (i.e. don't nag for a rating on every distill — only when a rate-worthy event
  seems to have gone unrated).
- Whether "underused" features (e.g. `harnez find`) have never been invoked this session despite
  opportunities (heuristic: manual `grep`/`ls issues` calls observed, or just periodic reminder
  regardless of evidence — keep v1 heuristic simple).

On each harnez invocation from a coding env, harnez should be able to return (via CLI stderr note,
or a structured field the calling agent's instructions tell it to surface) a short assessment:
gaps found + at most one proactive tip, not a wall of nagging text every time.

## 3. Implementation & Verification Plan

1. Design the session-state store: in-memory keyed by session id for a single process invocation
   won't work since harnez is invoked as a fresh process per call — needs a small persisted store
   (e.g. `~/.cache/harnez/sessions/<session_id>.json` or similar), consistent with how other
   session-scoped harnez state is already persisted (check existing patterns before adding a new
   one).
2. Add a hook point in the CLI dispatch layer that updates session state on every subcommand call
   and computes a cheap gap assessment.
3. Rate-limit the reminder itself (e.g. at most once per N calls or per M minutes) so this doesn't
   become exactly the kind of chatty, ignored signal that issue 181 is trying to fix for `rate`.
4. Add a unit test for the gap-detection heuristic (e.g. "N distill calls with zero rate calls
   triggers a reminder; a fresh session doesn't").
5. Verify with `go test ./...`; run `make install`.
