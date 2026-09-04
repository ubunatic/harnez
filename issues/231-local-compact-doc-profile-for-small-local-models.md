# 231 — local-compact doc profile for small local models

**Status**: Open — blocked on [[149-agent-specific-profiles-codex-async-wait-instruction]], tracking only
**Priority**: P3 (Low)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[166-compact-local-llm-doc-profile-and-go-rune-width-invariants]] (split from — original
ticket's Part B), [[165-two-phase-architect-patch-harness-for-small-local-models]] (same
`qwen-27b-local` source/motivation — small local models need a different interaction/instruction
shape than this project's docs and sprint workflow currently assume; likely the same profile work),
[[149-agent-specific-profiles-codex-async-wait-instruction]] (blocking dependency: the
per-agent/per-profile instruction mechanism this needs — `local-compact` would be a third profile
alongside Codex/Claude Code/etc.), [[151-on-demand-doc-lookup-vs-materialized-instructions-research]]
(directly on-point: materialized-instructions-vs-context-budget tradeoff for a low-context-budget
consumer), `config.yaml` (`agents_md`/`docs:` section)

---

## 1. Problem & Motivation

Split out of [[166]]: local 27B-class models suffer from "lost in the middle" degradation on
verbose documentation — long descriptive docs lead to hallucinated APIs and rule violations that a
more capable model wouldn't produce from the same doc. The fix proposed is a compact doc profile:
short, constraint-only rule templates instead of the project's current longer descriptive style.

The rules must read as explicit, imperative do/don't statements (e.g. "Don't slice a string by
byte index for display columns — use `[]rune` + a display-width library"), each scoped tightly to
the specific task/operation it governs, rather than paraphrased-shorter prose. A 120-line doc
still written as narrative paragraphs would not satisfy this.

166 was split because its Part A (Go rune/display-width invariants) was unblocked and shippable
now, while this profile mechanism has a real dependency ([[149]]) and should not be held hostage
to it, nor allowed to block Part A's already-demonstrated fix.

## 2. Requirements & Acceptance Criteria (from original source ticket, unedited)

- [ ] Add `target_profile: local-compact` option to `config.yaml`.
- [ ] Create minified rule templates (max 120 lines per doc) focused strictly on constraints rather
  than descriptions.
- [ ] Test that `harnez apply` outputs compact `AGENTS.md` without exceeding token budget.

(The source ticket's fourth item — Go rune/width invariants — shipped as [[166]] Part A and is not
repeated here.)

## 3. Scope Note

Same blocking dependency as [[165]]: needs [[149]]'s profile mechanism (or at least a converged
design) before `local-compact` can be added as a third profile rather than a bespoke `config.yaml`
flag with its own bespoke code path. No implementation should start until prioritized relative to
[[149]]/[[151]]/[[165]]; filed here for tracking only.

---

## Implementation Plan

**Stays blocked — do not start until [[149]]'s per-agent/per-profile mechanism exists.**

When it does:

1. Model `local-compact` as a *profile* in the existing `docs:` config structure, not a new
   top-level `target_profile` key with its own code path — a profile should select an alternate
   `source:` per doc entry (e.g. `source_compact: docs/compact/Go.md`), so `apply`/`init` reuse the
   existing copy machinery unchanged.
2. Author compact variants only for docs a small model actually needs at edit time (Go, Bash,
   Markdown) — not a mechanical shrink of all 15 doc entries.
3. Enforce the budget with a test, not a convention: a `TestCompactDocsUnderLineCap` in the config/
   docs test package walking every `docs/compact/*.md` and failing over 120 lines. That is the
   source ticket's line-cap acceptance criterion, made mechanical.

### Key decisions / tradeoffs

- **Profile as alternate `source:`, not a parallel renderer**: avoids a second doc pipeline. The
  cost is maintaining two texts per doc — accept that only for the 3 docs that matter.
- **Reuse `mattn/go-runewidth`**: already an indirect project dependency via `internal/usage`;
  mandating `golang.org/x/text` as well would add a dependency for no demonstrated need.

### Risks / open questions

- Real risk is doc drift: two versions of a doc (e.g. Go.md) diverge silently. No mitigation short
  of generating one from the other, which contradicts the "differently-shaped, not shortened"
  requirement. Worth naming explicitly before committing to this.
- Whether a 120-line imperative doc actually improves 27B-class output is unmeasured. Should not
  land without at least an informal A/B on one real task, or it is speculation with a maintenance
  bill.
- Design this alongside [[165]] rather than independently — both exist because of the same
  `qwen-27b-local` motivation and may collapse into one "local-compact"/"small-model" profile.

### Scope

**Large**, and blocked on [[149]].
