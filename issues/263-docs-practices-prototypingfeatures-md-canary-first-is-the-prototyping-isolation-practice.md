# 263 — docs/practices/PrototypingFeatures.md: canary-first IS the prototyping/isolation practice

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: docs/Canary.md, docs/practices/AgenticLoop.md, docs/practices/IssueTracking.md, root CLAUDE.md ("Docs Layout")

---

## 1. Problem & Motivation

`docs/Canary.md` ("Canary-First Development") already prescribes the practice of probing an
external mechanism or unknown in isolation, minimally, before building feature code on top of
it — and keeping the probe as a standing artifact. That *is*, functionally, a feature
prototyping / isolation practice. No doc currently states this explicitly, though. A newcomer
(or a future agent reasoning from generic industry patterns) might instead reach for a
separate "spike branch" or "feature flag" mechanism, not recognizing that canary-first already
covers the same need — creating a duplicate, uncoordinated pattern in this project.

This ticket proposes filing a short new doc that names the connection explicitly, so agents in
this repo default to canary-first for prototyping/isolation questions instead of inventing a
parallel mechanism.

## 2. Technical Specification / Findings

### Websearch: how other AI coding harnesses handle feature prototyping/isolation (2026)

- **Codex CLI / Codex cloud (OpenAI)** — uses a **git worktree model**: pick a worktree when you
  want isolation or plan to run several tasks in parallel; each isolated copy of the repo lets an
  agent iterate without touching the main tree. Cloud mode additionally runs each task in a
  network-disabled sandboxed container. Feature "maturity" labels (development/beta/stable) gate
  what's allowed in a production path, and autonomy is meant to graduate gradually (Suggest →
  Auto Edit → Full Auto) rather than jumping straight to unattended changes.
- **Devin (Cognition)** — `--sandbox` runs the CLI with OS-level isolation (writable-path
  allowlists, deny rules, optional network restriction); the March 2026 "Devin manages Devins"
  update runs hierarchical orchestration across **isolated VMs per sub-agent**. This is closer to
  environment isolation than to a prototyping *methodology* — it doesn't prescribe how to probe
  an unknown mechanism before committing to a design.
- **General industry framing ("tracer bullet" / "spike")** — several 2026 sources (Sourcegraph,
  Codescene, dev.to "Four Modalities") describe agent-assisted prototyping as a formalized,
  kept-runnable technical spike: agents spin up multiple alternative "tracer bullet"
  implementations in parallel, then compare which to keep. Distinction drawn from classic
  throwaway prototypes: tracer-bullet code is meant to evolve into the final feature rather than
  be discarded, whereas "vibe coding" throwaway prototypes are explicitly low-stakes and
  disposable (landing pages, tiny scripts, API exploration).
- **Feature flags for gradual rollout** — not really covered in what these harnesses document as
  agent-facing practice; it shows up mainly as a downstream *deployment* concern (Codex's
  maturity labels gating what ships) rather than something the coding agent itself manages during
  development.

### Honest comparison to canary-first

- **Covered by canary-first**: isolating an external/unknown mechanism, running it standalone,
  observing real output, documenting findings before building — this matches the "tracer
  bullet"/spike framing closely, and is stronger than most harness docs found (Canary.md keeps
  the probe as a durable, re-runnable artifact rather than a disposable spike, and explicitly
  distinguishes canary scope from full integration tests).
- **Not covered by canary-first, and worth an honest note rather than false parity**:
  - **Environment/process isolation mechanics** (worktrees, sandboxed containers, VM-per-agent)
    are a *runtime* isolation concern, not a design-probe concern — canary-first says nothing
    about where a canary itself runs relative to the main working tree. Worth a short
    cross-reference, not a rewrite.
  - **Feature flags for gradual/staged rollout** of an already-built feature — canary-first is a
    pre-build probe, not a rollout mechanism. Out of scope for canary-first and should be named
    as an explicit non-goal rather than silently absorbed into the "prototyping" framing.
  - **Parallel comparison of multiple competing implementations** (Sourcegraph's "spin up
    several tracer bullets, compare") — canary-first as written targets one mechanism at a time;
    it doesn't prescribe running N alternative designs side by side for comparison. Worth
    flagging as a gap rather than claiming canary-first already does this.

## 3. Implementation & Verification Plan

Scope: **a short doc** (this is explicitly not a candidate for a large/exhaustive doc — keep it
to roughly the length of a compact practices note, well under Canary.md's length).

- Create `docs/practices/PrototypingFeatures.md` (PascalCase per `docs/lang/Markdown.md`
  conventions for evergreen docs), stating the core thesis: this project's canary-first approach
  (`docs/Canary.md`) **is** its feature-prototyping/isolation practice — no separate spike-branch
  or feature-flag mechanism needs to be bolted on for the pre-build "does this actually work"
  question.
  - Briefly summarize the websearch findings above (condensed, not the full research dump).
  - State plainly where canary-first does **not** claim parity: environment/process isolation
    mechanics (worktrees/sandboxes), rollout-stage feature flags, and parallel side-by-side
    comparison of competing implementations are out of scope for canary-first and, if ever
    needed, are a separate concern.
- Cross-link the new doc from `docs/Canary.md` (e.g. a "See also" pointer) and from the root
  `CLAUDE.md` docs index (the `docs/practices/` bullet list alongside AgenticLoop.md and
  IssueTracking.md).
- **This ticket is for filing the proposal only.** Do not write
  `docs/practices/PrototypingFeatures.md` content in this session — that content is future work
  tracked by this ticket.

### Acceptance Criteria

- [ ] `docs/practices/PrototypingFeatures.md` exists, is short, and states the canary-first-is-
      prototyping thesis explicitly.
- [ ] Doc briefly notes findings from other harnesses (Codex CLI worktrees/sandboxes, Devin
      `--sandbox`/VM isolation, tracer-bullet/spike framing) and is honest about what canary-first
      does and does not cover (rollout feature flags, environment isolation mechanics, parallel
      multi-implementation comparison named as out of scope).
- [ ] Cross-linked from `docs/Canary.md` and from the `docs/practices/` bullet list in the root
      `CLAUDE.md` docs index.
- [ ] `harnez index` run after creation so `issues/README.md` reflects ticket closure.
