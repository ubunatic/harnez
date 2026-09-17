# 411 — Report session token use and final context size in canary-agenticloop-lite units

**Status**: Closed — implemented and verified: claude -p/agy -p --output-format json both expose parsed usage; measure-cost verb added, baseline measured (claude=60278, agy=40685 tokens/unit) and documented in results.md, along with the honest live-session-introspection limitation
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: `scripts/canary-agenticloop-lite/main.go`, `scripts/canary-agenticloop-lite/results.md`, [[362]]

---

## 1. Problem & Motivation

While building out `canary-agenticloop-lite` (issue 362 follow-up) in the current
session, the user asked to see the session's total token use and final context
size expressed "in canary-agenticloop-lite units" — i.e. as a multiple of the
cost of one `hello` fixture run (the cheap smoke-test fixture added this
session, which sends a single "reply with exactly the word: ready" prompt
against one doc variant/agent combo).

There is currently no way to answer that: the harness has no notion of its own
per-invocation token cost, and the host agent (this Claude Code session) has no
tool-exposed way to report its own live token usage or context size back to the
user on request. Both halves are missing.

## 2. Technical Specification / Findings

Two independent pieces would be needed to make "N canary-agenticloop-lite
units" a real, quotable unit:

1. **Per-fixture-run token cost measurement** in `canary-agenticloop-lite`
   itself — `claude -p` and `agy -p` invocations don't currently capture or
   print token usage (`main.go`'s `runClaude`/`runAgy` only return combined
   stdout+stderr text). `claude -p --output-format json` (or equivalent) may
   expose usage token counts; would need to check what `agy -p` exposes, if
   anything, and treat it as a canary-first probe rather than assumed.
2. **A queryable "cost so far" figure for the current session** — nothing in
   this Claude Code session currently reports live token/context usage on
   request. This may be a Claude Code product limitation with no in-scope fix
   here, or it may already exist as an undocumented capability — worth a
   canary-first probe before assuming either way.

Absent (2), the most honest deliverable is: measure the `hello` fixture's own
token cost empirically (one real `claude -p`/`agy -p` invocation, parsed usage
figures) and publish that as the "1 canary-agenticloop-lite unit" baseline in
`results.md`, without claiming the ability to report the *current* session's
own usage against it.

## 3. Implementation & Verification Plan

- Probe `claude -p --help` / `agy -p --help` for a machine-readable usage/token
  output flag; canary-first, do not assume JSON output exists before checking.
- If found, add a `--measure-cost` (or similar) mode to `canary-agenticloop-lite`
  that runs the `hello` fixture once per agent and prints parsed token usage,
  establishing the "1 unit" baseline.
- Investigate whether Claude Code (this host) exposes any live token-usage
  introspection today; if not, record that as an explicit known limitation in
  this ticket's resolution rather than silently dropping the request.
- Document the baseline and any host-side limitation in
  `scripts/canary-agenticloop-lite/results.md`.
- No automated test suite applies here (this is a manual probe + doc update);
  verification is the documented baseline number plus an honest statement of
  what could/couldn't be measured.
