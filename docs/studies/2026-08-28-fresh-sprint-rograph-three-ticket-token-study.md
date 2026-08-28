# Study: `/fresh-sprint` Token Economics — Three Dependent Tickets, One Run

**Date**: 2026-08-28
**Scope**: Empirical token/tool accounting for a single `/fresh-sprint` run executing three
dependency-ordered tickets (078 → 079 → 080) via sequential subagent handoffs, plus a modeled
counterfactual for running the same work in one continuous main-session context.
**Related Issues**: [078](../../issues/078-rograph-library-shared-bar-sparkline-renderer.md),
[079](../../issues/079-consistent-10-char-shrink-to-fit-graphs.md),
[080](../../issues/080-agent-box-layout-extraction.md)
**Related Docs**: [The 5-Phase Agentic Sprint & Independent Review Loop](2026-08-19-the-5-phase-agentic-sprint-and-independent-review-loop.md),
`~/.claude/commands/fresh-sprint.md`
**Status**: Completed — single-session observation, not a repeated benchmark

---

## 1. What Happened

Three tickets (`internal/rograph` extraction → consistent 10-char max-width → row-layout
extraction) were executed back-to-back under `/fresh-sprint`, each as a fresh `general-purpose`
subagent with a self-contained goal handoff. Each subagent's own tool-reported usage was captured
verbatim from its completion notification — this is measured data, not estimated:

| Ticket | Subagent tokens | Tool calls | Wall time | Outcome |
|:---|---:|---:|---:|:---|
| 078 — rograph extraction | 67,900 | 39 | 152.4s | Clean, no rework |
| 079 — MaxWidth=10 | 57,217 | 36 | 137.0s | Clean, one judgment call (see §4) |
| 080 — row-layout extraction | 75,255 | 33 | 188.6s | Clean, one judgment call (see §4) |
| **Total** | **200,372** | **108** | **478.0s (~8.0 min)** | 3/3 landed, 0 escalations |

All three produced a passing `go build ./... && go test ./...`, a `make install`, a ticket
close-out (`issues/NNN-*.md` + `issues/README.md` sync), a `harnez status` check, and a
conventional-commit — without any host intervention beyond the initial handoff and a post-hoc
diff skim. None required a second pass, none triggered the "escalate to full reviewer" branch of
`/fresh-sprint`'s confidence-gated review step.

Host-session cost (the orchestrator's own tokens: three delegation prompts, three post-completion
`git show --stat`/`Read` spot-checks, digesting three completion notifications) was not
separately metered by any tool available in this session — `harnez usage` reports
account-wide lifetime totals (2.59B tokens), not a per-session breakdown, so it cannot isolate
this run. Based on the size of the delegation prompts actually sent (roughly 700-950 words each)
and the small, targeted verification reads performed after each ticket (a `git show --stat`, two
~40-line file reads, one `git log`), host overhead for this run is estimated at roughly
**15,000-25,000 tokens** — an order of magnitude below the subagent total. This is an estimate,
not a measurement; no tool in this session exposes exact host-turn token counts.

---

## 2. Modeled Counterfactual: Same Work, One Continuous Session

No tool available in-session can *measure* what this would have cost run without subagent
isolation, since that run didn't happen. The following is a reasoned model, not an observation —
treat it as an order-of-magnitude argument, not a benchmark result.

**Why isolation should matter here specifically**: tickets 079 and 080 are declared dependencies
of the prior ticket(s). In a single continuous session, every tool call's *input* cost includes
the entire accumulated transcript up to that point — every prior file read, every prior
build/test log, every prior diff. Under subagent isolation, each ticket starts from a fresh
context and only pays for what it discovers on its own.

Rough model, using the measured per-ticket subagent cost as the "cost of the actual work" at
zero accumulated baggage:

- **Ticket 078 alone** would cost about the same either way (~68k tokens) — it's the first unit
  of work, nothing to carry yet.
- **Ticket 079**, run in-session right after 078, would carry roughly 078's full transcript
  (~68k tokens of accumulated context) as a standing tax on every one of its own ~36 tool calls,
  not just once — each tool call's input re-includes it. A conservative model (accumulated
  context counted once per ticket as fixed overhead, not per-call) puts 079 at roughly
  57k + 68k ≈ **125k tokens**.
- **Ticket 080** would carry both prior tickets (~068k + ~125k ≈ 193k accumulated), pushing its
  own 75k of work to roughly 75k + 193k ≈ **268k tokens**.

Summed: roughly **68k + 125k + 268k ≈ 460k tokens** for the same three tickets in one continuous
session — **more than double** the 200k actually spent via isolated subagents — before adding the
host's own orchestration overhead (negligible either way) or accounting for the fact that a real
single-session run would likely re-read `watch.go` in full multiple times per ticket (it's the
file all three tickets touch), which this per-call-counted-once model undercounts. **460k is a
floor estimate, not a ceiling.**

The mechanism matches the "Subagent Context Paradox" already documented in
[the 5-phase sprint study](2026-08-19-the-5-phase-agentic-sprint-and-independent-review-loop.md#takeaways):
isolation costs a fixed per-subagent initialization overhead but saves the compounding tax of
carrying prior tickets' full exploration transcript forward. For three *dependent*, non-trivial
tickets touching the same hot file, the savings dominate.

---

## 3. How the Handoff Guardrails Kept Each Subagent on the Rails

Each subagent got a fresh, self-contained prompt (no shared conversation memory with the host or
with each other) built from four kinds of guardrail, all present in every one of the three
handoffs:

1. **Explicit scope fences.** Each prompt named the ticket's non-goals verbatim from the ticket
   file (e.g. 078: "do not fix the width-24 hardcode... that's ticket 079's job"; 079: "no
   box/layout extraction... that's ticket 080"; 080: "History box is explicitly OUT OF SCOPE").
   This mattered concretely: 079's subagent, mid-implementation, found a spot where it *could*
   have threaded full box-width plumbing through `formatCPULine`/`formatGPULine`, but the fence
   ("prefer the simpler fix... unless the box-width plumbing is trivially already present") gave
   it a decision rule instead of leaving it to guess, and it chose the narrower fix.
2. **A "stop, don't force it" escape hatch.** Ticket 080's prompt explicitly said: if the
   extraction can't be done without changing behavior, stop and report rather than push through.
   This is what produced the one substantive judgment call of the run (§4) instead of a silent
   behavior change.
3. **Machine-checkable acceptance criteria, not prose approval.** Every handoff required
   `go build ./...`, `go test ./...`, and `make install` to pass before the subagent could declare
   done, plus `harnez status` for tracker consistency. This is what let the host skip a full
   independent-reviewer escalation per `/fresh-sprint`'s confidence-gated review step — the gate
   condition ("tests pass cleanly, routine refactor, no cross-subsystem ambiguity") was met by
   construction, not by the subagent's self-report.
4. **Ordering discipline enforced by the host, not the subagents.** The host ran the three
   subagents strictly sequentially, handing each one the prior ticket's actual commit SHA to read
   (`git show aa06ad7`) rather than re-describing it — the dependency chain (079 needs 078's
   `rograph.MaxWidth` surface; 080 needs both) was respected by construction rather than
   discovered by a subagent mid-task.

## 4. Where the Subagents Had (Mild) Problems

No subagent failed a build, failed a test, needed a retry, or produced output the host had to
send back for correction. The friction that did surface was exactly the kind the "stop, don't
force it" fence is designed to catch, and both instances were self-resolved and self-reported
rather than silently papered over:

- **Ticket 079** — `formatCPULine`/`formatGPULine` had no existing "available width" variable to
  thread `min(rograph.MaxWidth, availableWidth)` through cleanly. The subagent weighed full
  box-width plumbing against a narrower cap-only fix, chose the narrower one, and documented why
  (the box-render layer already truncates oversized lines, so the cap-at-10 fix is the
  correctness-relevant piece; full threading was deferred to 080's territory).
- **Ticket 080** — the ticket's own text asked for `formatCPULine`/`formatGPULine` to consume
  the new `RowLayout` helper, but those functions don't currently do bar-width negotiation at
  all — only label formatting. Extending `RowLayout` to them would have been a behavior change,
  which the ticket's own scope rules explicitly forbade for a "read-only refactor." The subagent
  applied the label-sharing half only, left the bar/trailing-info negotiation half to the one
  call site that actually needs it (`buildAgentBox`), and flagged the deviation from the ticket's
  literal wording in its report instead of silently under- or over-delivering.

Both cases are the same underlying failure mode *avoided*: a ticket's prose slightly outran what
was actually safe to do mechanically, and the "stop and report, don't force a behavior change"
constraint caught it before it became a silent regression. Neither required host correction after
the fact — the host's post-hoc diff review (`git show --stat` on all three commits) found nothing
to push back on.

---

## 5. Takeaways

1. **Measured isolation cost for this run: 200,372 subagent tokens across 108 tool calls and
   ~8 minutes of wall time, for three dependency-chained tickets landed with zero rework.**
2. **Modeled single-session cost is roughly 2x-plus higher (~460k+ tokens)** — not measured, but
   consistent with the compounding-context mechanism already known from prior sprint-loop
   research. The gap should widen, not narrow, with more dependent tickets in a chain.
3. Explicit non-goal fences plus a "stop and report, don't force it" escape hatch converted two
   places where a subagent could have quietly over-delivered (and introduced a behavior change
   under a "read-only refactor" ticket) into documented, reviewable judgment calls instead.
4. Machine-checkable acceptance criteria (build/test/install/tracker-status all passing) is what
   let `/fresh-sprint`'s confidence-gated review skip a full independent-reviewer subagent for all
   three tickets without weakening the review gate — the gate's condition was satisfied by tool
   output, not by the subagent's own narrative.
5. **Gap in tooling**: neither `harnez usage` nor any other available tool in this session exposes
   a per-session (as opposed to lifetime-account) token breakdown for the *host* orchestrator
   turn. The subagent-side numbers in this study are exact (tool-reported); the host-side and
   counterfactual numbers are estimates. A `harnez usage --session` view scoped to "tokens spent
   by this orchestrator turn, excluding subagent-reported totals" would turn §1's estimate and
   §2's model into measurements.
