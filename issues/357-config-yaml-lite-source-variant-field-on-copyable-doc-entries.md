# 357 — config.yaml: lite_source variant field on copyable doc entries

**Status**: Closed — schema/plumbing implemented and reviewed
**Priority**: P3 (Low)
**Severity**: Feature
**Category**: Templates / Docs / Token Efficiency
**Related**: [Issue 358](358-self-describing-variant-marker-so-drift-detection-tolerates-lite-docs.md) (hard prerequisite for shipping any lite doc), [Issue 359](359-pilot-agenticloop-lite-md-behavioral-canary-gate.md), [Issue 360](360-harnez-docs-variant-name-lite-full-thin-switch-verb.md), [Issue 231](231-local-compact-doc-profile-for-small-local-models.md) (inverse framing — same mechanism, different audience), [Issue 151](151-on-demand-doc-lookup-vs-materialized-instructions-research.md), [Issue 356](356-extract-anti-patterns-section-from-agenticloop-md-into-its-own-copyable-doc.md) (may become unnecessary if this lands), [Issue 323](323-docs-lang-bash-md-hard-references-docs-practices-agenticloop-md-instead-of-docs-agenticloop-md-alias.md), [Issue 249](249-copied-practice-docs-retain-dangling-and-inapplicable-dependencies.md)

---

## 1. Problem & Motivation

Issue 354 established that `@docs/X.md` references expand fully inline in Claude Code —
the whole file loads, not just the referenced section. The biggest copyable docs (e.g.
`docs/practices/AgenticLoop.md` at 26,844 bytes / ~9.1k tokens) cost this in full on
every session for every project that installed them, whether or not the session needs
more than a one-line reminder of a given rule.

Frontier models can often correctly expand a rule from just its name/tagline (e.g.
"Zero Zombie Guarantee: track and kill every background task/subagent") without needing
the full explanatory prose, case studies, and examples. A "lite" variant of the same doc
— same rules, taglined rather than explained — could be 15-20% of the size for a
capable-model audience, while the full doc remains available for less capable/local
models. This is the MVP for a mechanism to offer both, switchable, without duplicating
the doc-registry machinery.

Issue 356 assessed and rejected a structural split of `AgenticLoop.md` into multiple
files (high blast radius: three Go tests assert exact filenames, `config.yaml` registry
entries, path-aliasing fragility per issue 323, per-project drift per issue 249). This
ticket proposes a materially lower-blast-radius alternative: same file identity
(`ref`/`target`/`local` all unchanged), only the source content copied differs.

## 2. Proposed Fix

Add one new optional field, `lite_source`, on the existing `Language` struct
(`internal/claude/config.go:194`), sibling to the existing `source` field. Add a
resolver `func (l Language) SourceFor(variant string) string` that returns `lite_source`
when `variant == "lite"` and it's set, else falls back to `source`.

Wire the resolver into the three places that currently read `lang.Source` directly:
- `internal/claude/apply.go:1088` (`installDoc(cfg.FS, lang.Source, dst, …)`)
- `internal/claude/init.go:288` and `:726` (project copy + doc validation)
- `internal/claude/docs_capture.go:285-289` (drift detection baseline — coordinate with
  issue 358, which fixes the resulting drift-detection gap)

`ref`, `target`, and `local` are explicitly **out of scope** and must not change for a
lite variant — this is the property that keeps blast radius near zero: no new registry
entry, no `DependsOn` conflict logic, no new alias file, no test-filename churn.

Entries without `lite_source` set behave exactly as today under either variant value
(pure backward compatibility).

## 3. Acceptance Criteria

- [x] `Language` struct has an optional `lite_source` field; config schema docs updated.
- [x] `SourceFor(variant string) string` resolver implemented and unit-tested for both
      branches (lite-set, lite-unset).
- [x] All three read sites (`apply.go`, `init.go` ×2, `docs_capture.go`) use the resolver
      instead of `lang.Source` directly.
- [x] Regression test: an entry with no `lite_source` resolves to `source` under both
      `variant: "full"` and `variant: "lite"` — confirms zero behavior change for every
      existing doc entry until a lite variant is actually authored.
- [x] `go test ./...` and `scripts/smoke-test.sh` pass.
- [x] No lite doc content is authored in this ticket — schema/plumbing only. See issue
      359 for the pilot content.
