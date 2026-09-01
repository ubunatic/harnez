# 151 — Research: on-demand doc lookup vs. materialized instructions, and temporary per-repo agent profiles

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: N/A (research)
**Category**: Research / Architecture
**Related**: [[149-agent-specific-profiles-codex-async-wait-instruction]] (per-agent profile mechanism
this ticket assumes exists, then asks whether it should also apply per-repo and temporarily),
[[128-per-agent-full-system-prompt-self-audit-for-repetition]] (established that instruction
volume/repetition is a real, measured problem, not a hunch), [[130-instruction-distribution-audit-followups]]
(prior distribution audit — scoped to *what* gets materialized, this ticket asks whether
materialization itself is the right delivery model), [[116-tool-telemetry-schema-and-storage-layer]]
(`internal/telemetry`'s `tool_calls` table — candidate storage for "what did an agent ask/read"
tracking if this research recommends building it), `AGENTS.local.md` (existing precedent for a
local, untracked, repo-scoped instruction override), `docs/other/Spec.md`, `docs/Website.md`
(harnez's `apply`/`init` docs-materialization model)

> **Runtime note**: this ticket should be worked by a frontier-reasoning model (Opus, or the
> equivalent top-tier reasoning model on whatever agent runtime picks it up) — it's open-ended
> systems-design research with real trade-offs, not a scoped implementation task. Do not delegate
> it to a fast/cheap model.

## Problem

harnez's current instruction-delivery model is fully static and fully materialized: `apply`/`init`
write complete doc files (`AGENTS.md`, `~/.claude/CLAUDE.md`, `docs/lang/*.md`, etc.) into the
repo or the user's home directory, and the agent is expected to read them (directly, or via a
`@docs/...` reference it chooses to follow) as plain files. Two problems compound as the project
grows:

1. **Materialization has a footprint whether or not it's used.** Every generic doc bundled into
   `AGENTS.md` (Go conventions, Make conventions, Markdown conventions, etc.) sits in every
   project's context on every session, whether or not that session ever touches Go, Make, or
   Markdown. [[128]] already measured this is a real cost, not a hunch — repetition and prunability
   were quantified across two harnesses' clean-session system prompts.
2. **`@docs/...` references are advisory, not enforced.** An agent is expected to notice a doc
   reference and choose to read it. As the instruction surface grows, an agent — especially a
   smaller/weaker model that loses track of its own context earlier — has more competing signals
   and will more often skip a lookup it should have made. This is a soft failure mode: nothing
   errors, the agent just proceeds without information it needed.

The user's proposal, to investigate rather than commit to yet: replace some materialized doc
delivery with an **on-demand lookup mechanism** — a `harnez ask` command (or similar) an agent can
invoke to query for information instead of always having it pre-loaded. This raises a further,
partly independent idea: once *any* per-target scoping mechanism exists (see [[149]]'s per-agent
profiles), extend it to be **per-repository and temporary** too — instructions that live in a
`.harnez/` directory (parallel to the existing `AGENTS.local.md` precedent for local, repo-scoped,
untracked overrides) or similar, addable and removable without touching the user's home directory
or committing anything to the repo.

## What to research (not decide — that's the design doc's job)

1. **Does on-demand lookup actually reduce context cost, or just move it?** If an agent, once it
   knows "there's a Go doc," reads it in full on the first Go-related tool call and then keeps that
   content in its context for the rest of the session anyway (very plausible — this is normal
   agentic behavior, not a bug), an on-demand mechanism only helps sessions that never touch that
   topic at all. Investigate what fraction of a typical session's materialized-doc footprint is
   actually "never used this session" vs. "used once early and kept."
2. **Lookup mechanism shape.** Compare candidates: a `harnez ask <query>` CLI the agent invokes as
   a tool call (works today, no new agent-side capability needed); a `harnez ask --list` topic
   index; a small local database (or reuse of `internal/telemetry`'s existing DuckDB, see [[116]]);
   an MCP server exposing the same lookups as an MCP tool instead of a raw CLI invocation. Each has
   different discoverability, latency, and "will the agent actually call it" characteristics —
   research what's known about how differently-sized/differently-branded agents (Claude Code, agy,
   Codex, Gemini) treat an available-but-optional tool vs. materialized context they can't avoid
   reading. This is explicitly *not* fully known going in (see the user's own uncertainty about
   agent read behavior) — say so plainly in the design doc rather than guessing confidently.
3. **Full doc vs. section-level retrieval.** If agents tend to pull in a whole doc once they decide
   to look at it, a section-granular lookup API may add complexity for no benefit over "fetch the
   whole file on demand." Investigate whether this project's existing docs are already
   section-addressable (headers, `**Scope**:` fields — cf. `docs/README.md`'s `harnez index`
   topic-extraction logic from [[148]]) or would need restructuring.
4. **Per-repo, temporary profile layering.** Assess whether a `.harnez/` local directory (docs, or
   a lookup DB) is a sound extension of the existing `AGENTS.local.md` local-override precedent —
   same trust model (untracked, repo-scoped, no home-directory or committed-repo changes), or does
   it need different guarantees (e.g., a TTL/expiry so a "temporary" instruction doesn't silently
   become permanent)? Consider interaction with [[149]]'s per-agent profile mechanism: is
   per-repo-per-agent a natural generalization of per-agent, or a genuinely separate axis that
   needs its own design?
5. **Backward compatibility constraint (hard requirement, not a research question).** Any
   not-yet-materialized/on-demand lookup feature this research proposes must be **additive and
   optional** — existing materialized docs keep working exactly as they do today. Do not propose
   replacing materialization outright; propose it as an alternative delivery path, validated on a
   small scale first.
6. **Pilot scope, if the design doc recommends proceeding.** Identify 1-2 of the most generic,
   least-session-critical bundled docs (the user suggested `Make.md` and `Markdown.md` as
   candidates — cheap to re-fetch, rarely load-bearing for the current task) as a controlled
   experiment: move them out of default materialization, behind the proposed lookup mechanism only,
   and observe whether agent behavior degrades (missed conventions, wrong formatting) before
   considering a wider rollout. This should be scoped as a *follow-up ticket*, not built inside
   this research ticket.
7. **Usage tracking for the lookup mechanism itself, if built.** If a `harnez ask`-style command
   ships, its calls are exactly the kind of event `internal/telemetry`'s `tool_calls` table already
   exists to record (see [[116]], and the existing `harnez rate`/`harnez stats` machinery) — assess
   whether that table can carry lookup-query records directly or needs a schema extension, rather
   than inventing a second telemetry store.
8. **Same-question / same-session dedup, with a strict "meta stays smaller than content" bound.**
   Investigate whether a session (identified by `session_id`, per the existing convention in
   [[121]]/[[117]]/[[118]]) asking the same lookup question twice can be detected and short-circuited
   — either by returning a compact "already provided this, see above" notice instead of re-sending
   the full content, or by re-sending the content with a short annotation. Explicitly flag the two
   known hard cases rather than hand-waving them: (a) a forked/spawned subagent inherits the
   parent's context in some harnesses but not others, so "has this session already read X" may not
   be answerable the same way across Claude Code/agy/Codex; (b) the mechanism must never let its own
   bookkeeping (warnings, "already asked" annotations, provenance metadata) outweigh the size of the
   actual content being tracked — a research finding that dedup overhead approaches or exceeds
   typical answer size is itself a valid conclusion, not a failure to solve the problem.

## Deliverable

A design document (not code, not a committed schema) covering the above, written for the user's
review before any implementation ticket is filed. Cover:
- The options considered for each research question above, with trade-offs stated plainly —
  including "we don't know, here's how we'd find out" where genuine uncertainty exists (e.g. cross-
  harness agent read behavior).
- A recommendation, clearly marked as a recommendation the user can accept, reject, or amend — not
  a decision already made.
- An explicit backward-compatibility statement confirming existing materialized-doc delivery is
  unaffected by whatever is recommended.
- A short "what a pilot would look like" section per item 6 above, scoped small enough to falsify
  quickly if it doesn't work.

Once the user reviews and gives feedback on the design document, file concrete implementation
ticket(s) from it — do not implement anything from this ticket directly.

## Out of Scope

- Any actual code, schema, or command implementation — this is research + a design doc only.
- Deciding the final mechanism (CLI vs. MCP vs. DB) — the design doc presents options, the user
  decides.
- Migrating any doc out of default materialization for real — the pilot in item 6 is scoped as a
  follow-up ticket, contingent on the user accepting the design doc's recommendation.
