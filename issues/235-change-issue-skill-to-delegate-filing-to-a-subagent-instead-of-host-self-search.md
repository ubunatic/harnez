# 235 — Change /issue skill to delegate filing to a subagent instead of host self-search

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `commands/issue.md` (the skill being changed), `commands/harnez-sync.md` /
`commands/sprint.md` / `commands/fresh-sprint.md` (existing precedent for "dispatch a fresh
subagent, host stays responsive" skill structure), `AGENTS.md`'s "Background Tasks & Process
Hygiene" section ("Subagent handoff must not block the main chat... do not immediately wait on
the child agent unless the user explicitly asks... or the next user-visible integration step
truly cannot proceed without the result")

---

## 1. Problem & Motivation

`commands/issue.md`'s current TL;DR has the **host agent itself** perform every step of filing a
ticket: search for duplicates (`harnez find`), reserve a number, draft the ticket content, and
commit. That means the host spends its own context and tool calls on exploratory work — searching
the tracker, reading nearby tickets for conventions, drafting prose — every single time a filing
request comes up mid-conversation, even though that work is naturally ephemeral and disposable
(it doesn't need to stay in the host's context once the ticket is filed).

This is inconsistent with how this session has handled comparably-sized work elsewhere: dev work,
design review, and doc sweeps have all been dispatched to fresh or forked subagents (see
`commands/harnez-sync.md`, `commands/sprint.md`, `commands/fresh-sprint.md`) specifically to keep
the host's own context focused on orchestration rather than execution detail. `/issue` is the one
skill that still does its work inline on the host.

## 2. Proposed Change

Change `/issue`'s workflow so the host:

1. Does **not** run `harnez find`/duplicate search itself, and does **not** dig back through the
   conversation transcript or the repo to reconstruct context.
2. Instead, composes a **self-contained handoff prompt** — everything relevant the host already
   knows from the session so far (the concrete ask, any motivating context/reasoning already
   discussed, related files/tickets/commits already touched or mentioned) — and dispatches a
   fresh subagent to do the actual filing: duplicate search, drafting, `harnez issues new`,
   writing the ticket content, and `harnez issues open --commit`.
3. The subagent is a fresh dispatch (no conversation memory of its own, per this harness's
   `Agent`-tool semantics — "briefed like a smart colleague who just walked into the room"), so
   the handoff prompt must be complete on its own: it cannot say "as discussed above" or "per the
   last message," it must actually state what was discussed.

This mirrors — and should explicitly cite as precedent — this harness's general dispatch pattern:
give the subagent full context in the prompt itself (never "go re-derive it"), let it do the
search/draft/file work independently, and decide separately whether the host waits synchronously
for the ticket number or stays responsive and reports back later (per `AGENTS.md`'s existing
non-blocking-by-default rule, tempered by "unless the next user-visible step truly cannot proceed
without the result" — filing a ticket is often small/fast enough that synchronous waiting may be
the right default here specifically; this is a judgment call for whoever implements, not
prescribed by this ticket).

## 3. Open Questions

- **Sync vs. async wait**: should the host block on the subagent's filing result (to report the
  ticket number back immediately, which is often what the user wants next) or dispatch-and-stay-
  responsive like `harnez-sync`? Ticket filing is typically fast; blocking may be the more useful
  default here even though it diverges from other skills' async-by-default framing. Decide with
  evidence, not by copying `harnez-sync`'s pattern uncritically.
- **What exactly goes in the handoff prompt**: the ticket says "what it knows from the session so
  far" — needs a concrete minimum bar (the raw ask, verbatim where useful; any files/commits/
  tickets already touched this session that are relevant; any explicit constraints the user
  stated) versus what's still fine to leave to the subagent to discover itself (duplicate search,
  nearby-ticket convention inspection — these are exactly the exploratory steps that belong to
  the subagent, not the host, per this ticket's whole premise).
- **Does this apply to `/roadmap` (234) too?** That skill was just filed/implemented with its own
  "dispatch a fresh subagent" structure already built in from the start. Worth checking `/issue`
  and `/roadmap` end up with a consistent handoff-prompt convention rather than two divergent
  ad hoc shapes, since both are host-triggered filing/synthesis skills that dispatch subagents.

## 4. Non-Goals

- Not proposing to change what a filed ticket looks like (metadata schema, content structure) —
  only *who* (host vs. subagent) does the search/draft/file work.
- Not proposing to remove the host's ability to file directly in a pinch — just changing the
  documented default workflow.

No implementation plan yet — filed to capture the idea.
