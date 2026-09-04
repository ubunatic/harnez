# 236 — Add a /discovery skill: gap analysis between product use cases and current implementation

**Status**: Closed — implemented; commands/discovery.md + config.yaml registration, reviewed and verified
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[234-add-a-roadmap-skill-product-manager-style-roadmap-creation-update-from-open-issues]]
(`/roadmap` — the sibling skill this complements; roadmap sequences the *existing* backlog,
discovery finds what should be *added* to it), [[235-change-issue-skill-to-delegate-filing-to-a-subagent-instead-of-host-self-search]]
(the delegation/handoff pattern this skill's output should probably plug into, if it proposes
filing anything), `commands/roadmap.md` (structural precedent — same "read the repo's own
identity docs to establish its value axis" step reused here), `commands/issue.md`

---

## 1. Problem & Motivation

`/roadmap` (234) is explicitly read-only against the issue tracker and only sequences tickets that
already exist. When its synthesis step surfaces something the product clearly needs but nothing
currently tracks, the skill has no path to act on it — it either drops the observation or (at best)
mentions it in roadmap prose with nowhere for the reader to act on it.

This is a distinct discipline from roadmapping, commonly called **product discovery** (or
**opportunity discovery**) — identifying unmet needs and missing capabilities *before* they become
backlog items — as opposed to backlog grooming/roadmapping, which works from tickets that already
exist. The specific analytical move (comparing a product's current capabilities against its stated
purpose to spot what's missing) is usually called **gap analysis**.

harnez currently has no skill for this half of the loop: `/roadmap` orders what's already known to
be needed, `/issue` files a specific idea once someone has already thought of it, but nothing
actively looks for ideas nobody has filed yet.

## 2. Proposed Shape

A new `commands/discovery.md` skill (`/discovery`, optionally `/discovery <repo>`), structured
similarly to `/roadmap`'s dispatch-a-fresh-subagent shape:

1. **Establish the product's use cases** — read the target repo's own identity docs
   (`README.md`, `AGENTS.md`, `CONTEXT.md`) for what the product is actually for. (Same step
   `/roadmap` already does — consider whether this should be a shared instruction fragment rather
   than duplicated prose between the two skill files, though that's an implementation detail, not
   a blocker.)
2. **Establish what the product actually does today** — not what the backlog says is planned, the
   *real, currently-implemented* surface. For a CLI like harnez this likely means enumerating
   actual commands/flags (`harnez --help` and subcommand `--help` output, or a skim of
   `cmd/harnez/*.go`'s command registrations) rather than reading docs about intended behavior,
   since docs can be aspirational or stale (see this session's own doc-drift discoveries).
3. **Find the gap** — identify capabilities a user pursuing the stated use cases would reasonably
   expect, that neither exist in the implementation nor are already tracked by an open *or closed*
   ticket (check closed tickets too, so it doesn't re-propose something already deliberately
   rejected — e.g. this session closed 123 as obsolete for a reason; discovery shouldn't
   resurrect it blind).
4. **Report candidates, don't auto-file them.** Output a report of gap candidates with reasoning
   (why this gap matters to the stated use cases, why it isn't already covered). Filing any of
   them as real tickets is a separate, human-gated step — likely by handing the chosen candidates
   to `/issue`'s workflow (see 235's delegation pattern once implemented) rather than this skill
   writing directly to `issues/`.

## 3. Open Questions

- **Scope of "what the product does today"**: for a Go CLI, command/flag enumeration is
  reasonably mechanical. For other kinds of projects (a GUI app, a library, `voxi`'s
  dictation pipeline) this step needs different concrete guidance — decide per-repo or leave the
  method itself to the dispatched subagent's judgment, similar to how `/roadmap` leaves "value
  axis" derivation open-ended rather than prescribing a fixed method.
- **Relationship to `/roadmap` — resolved: stays separate.** Different cadences (discovery is
  occasional/exploratory, roadmap sequencing is more routine) and different output review needs
  (discovery candidates need human judgment before becoming tickets; roadmap output doesn't touch
  the tracker at all). Implement as its own `commands/discovery.md`, not a mode/flag of `/roadmap`.
- **False-positive risk**: gap analysis from an LLM against a product's docs is prone to proposing
  generic "nice to have" features that don't actually fit the product's real constraints (see this
  repo's own `docs/AgenticLoop.md` anti-pattern culture around not inventing scope). The skill
  should probably instruct skepticism explicitly — e.g. require the candidate to tie to a concrete,
  named use case, not a generic "most products have X."

## 4. Non-Goals

- Not a replacement for `/roadmap` — sequencing existing tickets stays roadmap's job.
- Not proposing this skill file tickets automatically — human review gates any filing.
- Not proposing a fixed scoring rubric for "what counts as a gap" — same philosophy as 234's
  refusal to hardcode a value-judgment formula.

No implementation plan yet — filed to capture the idea from this session's own conversation about
the roadmap skill's read-only limitation.
