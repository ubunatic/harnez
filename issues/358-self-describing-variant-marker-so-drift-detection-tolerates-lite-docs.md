# 358 — Self-describing variant marker so drift detection tolerates lite docs

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Bug (prerequisite — blocks shipping any lite doc)
**Category**: Templates / Docs
**Related**: [Issue 357](357-config-yaml-lite-source-variant-field-on-copyable-doc-entries.md) (adds the variant field this depends on), [Issue 359](359-pilot-agenticloop-lite-md-behavioral-canary-gate.md), [Issue 249](249-copied-practice-docs-retain-dangling-and-inapplicable-dependencies.md)

---

## 1. Problem & Motivation

`internal/claude/docs_capture.go:285-289` detects doc drift by diffing an installed
local doc (e.g. `./docs/AgenticLoop.md`) against the registry's `lang.Source` content.
Once issue 357 adds a `lite_source` variant, a project that installed the lite variant
will diff against the full-doc `source` by default and report **permanent, unfixable
drift** — the installed lite content will never match the full source it's being
compared against.

This is a hard prerequisite: shipping any lite doc (issue 359) without this fix makes
`harnez status`/`harnez diff` permanently noisy for every project that opts into lite,
which is worse than not offering the feature.

## 2. Proposed Fix

Add a self-describing marker recognized alongside the existing `harnez:begin/end/stop`
markers in `internal/markdown` — e.g. `<!-- harnez:variant=lite -->` as the first line
of a lite-variant doc's content (present in the source file itself, so it round-trips
through installation).

`docs_capture.go`'s drift check reads the marker from the **installed** local doc first,
resolves the matching source via issue 357's `SourceFor(variant)`, and diffs against
that — not unconditionally against `source`. A doc with no marker (the common case,
every existing doc) behaves exactly as today (implicit `variant=full`).

## 3. Acceptance Criteria

- [ ] `<!-- harnez:variant=lite -->` (or equivalent) marker recognized by
      `internal/markdown` alongside existing managed-block markers.
- [ ] `docs_capture.go` resolves drift baseline from the installed doc's own variant
      marker, not a fixed `source` reference.
- [ ] Test: a lite-installed doc reports clean drift status in `harnez status`.
- [ ] Test: a locally-edited lite doc (content changed after install) still correctly
      reports drift — the marker-based resolution doesn't mask real edits.
- [ ] Test: an unmarked (full-variant) doc's drift detection is unchanged from current
      behavior — no regression for the existing doc set.
- [ ] `go test ./...` and `scripts/smoke-test.sh` pass.
