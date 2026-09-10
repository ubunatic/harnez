# 166 — Go rune/display-width invariants in docs/lang/Go.md

**Status**: Closed — resolved: documented rune handling, terminal cell width, ANSI stripping, and TUI width assertions; compact profile remains in #231
**Priority**: P3 (Low)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `docs/lang/Go.md`, `internal/usage/indicatorsspec.go`, `internal/usage/watch_test.go`,
[[231-local-compact-doc-profile-for-small-local-models]] (split off — this ticket's original Part
B, the `local-compact` profile mechanism)

---

## 1. Problem & Motivation

External ticket received (source: pasted YAML ticket content) called out that local 27B-class
models suffer "lost in the middle" degradation on verbose documentation, and proposed a compact
doc profile as the fix. That profile-mechanism work has been split into [[231]] since it is
blocked on [[149]]'s per-agent profile mechanism, while this ticket's other requirement — Go
rune/display-width invariants — is independent of any profile mechanism and shippable now.

`docs/lang/Go.md` (currently 77 lines) has **no existing rune/display-width guidance** — the
string-indexing/`len()`-for-columns failure mode is a real, currently undocumented gap. This
project's own `internal/usage` TUI code has hit exactly this class of bug before (see the prior
TUI rendering postmortem in `docs/studies/`), and `internal/usage` already depends on
`github.com/mattn/go-runewidth` (confirmed in `internal/usage/indicatorsspec.go:11` and
`internal/usage/watch_test.go:13`), so the dependency choice is already made and needs no new
decision.

## 2. Requirements & Acceptance Criteria

- [ ] Enrich Go guidelines with explicit TUI and string handling invariants:
  - Strict ban on direct string indexing `s[i:j]` for anything a human reads or a column-aligned
    layout consumes.
  - Strict ban on `len(s)` for visual column layout.
  - Enforcement of `[]rune` conversions and `mattn/go-runewidth` for terminal column width.

---

## Implementation Plan

Independent of any profile mechanism, and §1 already establishes the gap is real.

1. Add a `## Strings, Runes & Terminal Width` section to `docs/lang/Go.md` (insert after
   `## Types & Style`, before `## Output Discipline` — it is a typing/style rule, and
   `## Output Discipline` then reads as its consumer). Write it as imperative do/don't lines,
   ~10-14 lines:
   - Don't index or slice a string by byte offset (`s[i:j]`) for anything a human reads or a
     column-aligned layout consumes.
   - Don't use `len(s)` as a visual column count — it is bytes, not cells.
   - Do use `[]rune(s)` for character-count logic, and `runewidth.StringWidth` for terminal
     column width (CJK/emoji are 2 cells; combining marks are 0).
   - Do strip ANSI escapes before measuring width (this repo's own `stripANSI` prior art).
   - Do assert rendered width in tests, not rune count (point at the existing
     `internal/usage/watch_test.go` / `indicatorsspec_test.go` pattern as the canonical example).
2. Update the `golang` entry's `hint:` in `config.yaml` (~line 364) to mention the rune/width rule,
   so the one-line hint surfaced to agents that don't open the doc still carries the constraint.
3. Verify: `go test ./...`, then `harnez diff` to confirm the doc/hint delta is the only change
   `apply` would make. Do not run `make install` as part of verification of a docs-only change
   beyond what the repo's normal workflow already requires.

Closes the concrete, demonstrated failure mode.

### Design decisions / tradeoffs

- **Reuse `mattn/go-runewidth`**: already an indirect project dependency via `internal/usage`;
  mandating `golang.org/x/text` as well (as the source ticket suggested) would add a dependency
  for no demonstrated need.

### Scope

**Small** — one doc section, one hint line, no code.
