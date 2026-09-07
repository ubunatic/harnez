# 283 — Reassess mandatory fresh-subagent issue filing against direct host filing

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[235-change-issue-skill-to-delegate-filing-to-a-subagent-instead-of-host-self-search]], [[241-session-tree-tool-call-efficiency-audit-command-and-agent-skill]], `commands/issue.md`, `docs/feedback/2026-08-28-subagent-handoff-responsiveness-and-load-memory.md`, `docs/studies/2026-08-28-fresh-sprint-rograph-three-ticket-token-study.md`, commits `38baeec`, `0f0a690`, `c99a437`

---

## 1. Problem & Motivation

Issue 235 changed `/issue` from direct host filing to a mandatory fresh-subagent workflow. The
stated goal was to isolate disposable duplicate-search and drafting context while keeping the host
responsive. That tradeoff was plausible for larger delegated tasks, but recent use suggests the
fixed delegation cost may dominate a small, bounded issue-filing workflow.

In the 2026-09-07 session, a reused subagent filed two earlier tickets (`281` and `282`; commits
`0f0a690` at 20:45 and `c99a437` at 21:02). Delegation required a self-contained handoff that
repeated session context and delayed delivery of the ticket result. A later attempt to satisfy the
skill's *fresh-subagent* rule by inheriting conversation context failed with:

```text
invalid thread-store request: no rollout found
```

It had to be retried with `fork_turns=none`, which then required the relevant context to be copied
into the handoff explicitly. This is concrete evidence of extra latency and duplicated context,
though exact token and billing data are unavailable. The repository's telemetry currently records
one Codex `spawn_agent` failure for project `harnez` (score 1/failure), but does not correlate it to
an issue-file operation or capture end-to-end delegation latency and token totals.

Reassess whether fresh delegation should remain mandatory. Direct filing may be cheaper and faster
for a host that already has the relevant context and issue conventions loaded, even though the
ticket search and draft add some short-lived material to the host transcript.

## 2. Findings and Tradeoffs to Assess

- **Latency:** direct filing is three short local steps (duplicate search, reserve/write, open and
  commit). Delegation adds scheduling, startup, handoff, notification, and sometimes retry time.
  The 281/282 commit timestamps are elapsed-time proxies, not clean measurements: they are 17
  minutes apart and cannot isolate filing duration or idle/user time.
- **Context and token cost:** a fresh child avoids retaining tracker exploration in the host, but
  must ingest baseline instructions plus a duplicated self-contained handoff. When inherited
  context is used, it may clone far more history than issue filing needs; when `fork_turns=none` is
  used, the host must restate relevant history. Provider-reported per-operation token and billing
  data are not available, so no cost winner can yet be claimed empirically.
- **Responsiveness:** asynchronous delegation lets the host answer unrelated questions while filing
  proceeds, as required by the documented handoff rule. For a terminal request whose desired result
  is the ticket number/commit, however, returning before completion is not useful and a long wait is
  user-visible overhead. Direct filing also keeps the host responsive if the local workflow finishes
  quickly.
- **Correctness and isolation:** a fresh agent can independently search duplicates, choose metadata,
  and avoid contaminating the host's working context. It also introduces handoff-loss risk, may miss
  nuances not copied into the prompt, and adds another writer in a shared worktree. `harnez issues
  new` already provides atomic number reservation, while path-scoped commits protect unrelated work;
  these safeguards do not inherently require another agent.
- **Scale matters:** the fresh-sprint study measured isolation benefits for three substantial code
  tickets (200,372 child tokens and about eight minutes total, versus a modeled 460k-token continuous
  session). That evidence supports isolation for multi-step implementation, not automatically for a
  one-ticket administrative workflow. Its host cost and counterfactual were estimates.

Direct host filing should be preferred when the request is one ticket, the relevant context and
conventions are already present, duplicate search is narrow, no independent product judgment is
needed, and the result is immediately required. Fresh delegation remains attractive for batches,
wide or ambiguous discovery, context-heavy synthesis, a nearly saturated host context, or when the
host has genuinely useful concurrent work and does not need the ticket result next.

## 3. Proposed Change and Measurement Plan

1. Change `commands/issue.md` from mandatory fresh delegation to a decision rule, with direct host
   filing as the default for a single bounded ticket and delegation for the conditions above. Keep
   staging/commit scope and duplicate-search requirements identical in both paths.
2. Run a small paired benchmark (at least 10 representative filings per path, including simple and
   exploratory tickets) and record:
   - request-to-reservation, request-to-commit, and request-to-user-result wall time;
   - host input/output tokens, child input/output tokens, cached tokens, and billed cost where the
     runtime exposes them;
   - handoff prompt bytes/tokens and inherited-context size;
   - spawn attempts, retries/errors, tool calls, duplicate misses, metadata/lint corrections, and
     post-filing edits;
   - whether the host served another user turn before filing completed.
3. Add an issue-filing operation/correlation ID to telemetry so spawn, `harnez find`, reservation,
   index, commit, and completion can be grouped without reading raw transcripts. Record runtime and
   model because spawn semantics and costs differ across harnesses. Never infer provider billing
   from the existing four-bytes-per-token heuristic.
4. Define a decision threshold from measurements, for example: delegate only when predicted filing
   work exceeds the observed spawn/handoff break-even, more than one ticket is requested, duplicate
   discovery is broad, or independent review has explicit value.

## 4. Acceptance Criteria

- [ ] A measured comparison covers both direct and delegated issue filing with the metrics above.
- [ ] The study distinguishes measured values from elapsed-time proxies and modeled estimates.
- [ ] `commands/issue.md` documents an evidence-based choice instead of unconditional delegation.
- [ ] Direct and delegated paths both preserve duplicate search, atomic reservation, tracker lint,
      generated-index synchronization, immediate path-scoped commit, and unrelated worktree state.
- [ ] Tests or canary evidence cover a single simple ticket, an exploratory ticket, a batch, and a
      spawn failure/fallback.
