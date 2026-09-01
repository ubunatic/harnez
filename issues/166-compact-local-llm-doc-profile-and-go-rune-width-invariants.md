# 166 — Compact Local-LLM Doc Profile with Explicit Do/Don't Rules (+ Go Rune/Width Invariants)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[165-two-phase-architect-patch-harness-for-small-local-models]] (same source/target —
`qwen-27b-local` — and same underlying constraint: small local models degrade on this project's
current doc style; likely the same profile work, not two separate mechanisms),
[[149-agent-specific-profiles-codex-async-wait-instruction]] (per-agent/per-profile instruction
mechanism this would need — `target_profile: local-compact` in the source ticket is effectively a
proposed third profile alongside Codex/Claude Code/etc.),
[[151-on-demand-doc-lookup-vs-materialized-instructions-research]] (directly on-point: this is a
materialized-instructions-vs-context-budget tradeoff for a specific low-context-budget consumer),
`docs/lang/Go.md`, `config.yaml` (`agents_md` section)

---

## 1. Problem & Motivation

External ticket received (source: pasted YAML ticket content — not yet triaged against harnez's
own priority/severity conventions; original fields were `type: feature`, `priority: high`,
`target: qwen-27b-local`, re-scored below per harnez's own schema):

Local 27B-class models suffer from "lost in the middle" degradation on verbose documentation —
long descriptive docs lead to hallucinated APIs and rule violations that a more capable model
wouldn't produce from the same doc. The fix proposed is a compact doc profile: short, constraint-
only rule templates instead of the project's current longer descriptive style.

The user added an explicit style requirement beyond the source ticket: for this profile, docs
should be **super clear do's and don'ts instead of long text**, and **specific to the task at
hand** — not a generically shortened version of the existing prose docs, but a differently-shaped
document (imperative constraint list vs. descriptive explanation).

This is the same `qwen-27b-local`-origin/motivation as
[[165-two-phase-architect-patch-harness-for-small-local-models]] — both tickets exist because a
weaker local model needs a different interaction/instruction shape than the frontier models this
project's docs and sprint workflow currently assume. They should likely be designed together under
one "local-compact" or "small-model" profile rather than as two independent mechanisms.

## 2. Requirements & Acceptance Criteria (from source ticket, unedited)

- [ ] Add `target_profile: local-compact` option to `config.yaml`.
- [ ] Create minified rule templates (max 120 lines per doc) focused strictly on constraints rather
  than descriptions.
- [ ] Enrich Go guidelines with explicit TUI and string handling invariants:
  - Strict ban on direct string indexing `s[i:j]`.
  - Strict ban on `len(s)` for visual column layout.
  - Enforcement of `[]rune` conversions and `golang.org/x/text` / `mattn/go-runewidth`.
- [ ] Test that `harnez apply` outputs compact `AGENTS.md` without exceeding token budget.

## 3. Additional Requirement (from user, this session)

- The compact profile's rules must read as explicit, imperative do/don't statements (e.g. "Don't
  slice a string by byte index for display columns — use `[]rune` + a display-width library"), each
  scoped tightly to the specific task/operation it governs, rather than paraphrased-shorter prose.
  This is a distinct requirement from just hitting a line-count budget — a 120-line doc that's still
  written as narrative paragraphs would satisfy the source ticket's line cap but not this
  requirement.

## 4. Notes / Existing Gaps

- `docs/lang/Go.md` (currently 77 lines) has **no existing rune/display-width guidance** — the
  string-indexing/`len()`-for-columns failure mode the source ticket calls out is a real, currently
  undocumented gap, independent of which profile format eventually ships. Worth fixing in
  `docs/lang/Go.md` itself regardless of the local-compact profile's fate, since this project's own
  `internal/usage` TUI code has hit exactly this class of bug before (see prior TUI rendering
  postmortem in `docs/studies/`).
- `internal/usage` already depends on rune-width handling for its box-drawing layout; check whether
  it already uses `golang.org/x/text`/`mattn/go-runewidth` or an equivalent before mandating a new
  dependency choice in the doc.

## 5. Open Questions / Scope Note

- Same blocking dependency as [[165]]: needs [[149]]'s profile mechanism (or at least a converged
  design) before "local-compact" can be added as a third profile rather than a bespoke
  `config.yaml` flag with its own bespoke code path.
- No implementation should start until prioritized relative to [[149]]/[[151]]/[[165]]; filed here
  for tracking only.
