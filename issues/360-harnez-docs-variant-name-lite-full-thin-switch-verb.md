# 360 — harnez docs variant <name> lite|full — thin switch verb

**Status**: Closed — flags + docs variant verb implemented, reviewed, and manually verified end-to-end
**Priority**: P3 (Low)
**Severity**: Feature
**Category**: CLI / Templates
**Related**: [Issue 357](357-config-yaml-lite-source-variant-field-on-copyable-doc-entries.md) (prerequisite: variant field), [Issue 358](358-self-describing-variant-marker-so-drift-detection-tolerates-lite-docs.md) (prerequisite: drift detection), [Issue 359](359-pilot-agenticloop-lite-md-behavioral-canary-gate.md) (needs this or a manual toggle to install), [Issue 231](231-local-compact-doc-profile-for-small-local-models.md) (this supersedes 231's mechanism half), [docs/CLIDesign.md](../docs/CLIDesign.md) (apply/init separation this must respect)

---

## 1. Problem & Motivation

Issues 357-359 add the ability for a copyable doc to have a lite content variant, but
nothing yet lets a user or project switch which variant is installed without re-running
the full `init`/`apply` flow (which touches far more than the one doc file — Makefile,
language conventions, other docs, etc.).

Separately, issue 231 ("local-compact doc profile for small local models") proposed a
`target_profile: local-compact` mechanism for the inverse audience (weak models needing
short docs, vs. this ticket's strong models tolerating tagline-only docs). The
file-selection machinery — pick between two content variants of the same registered doc
— is identical in both cases. This ticket's mechanism should serve both audiences,
making 231's proposed mechanism redundant.

## 2. Proposed Fix

1. Add a `--variant lite|full` flag to `init` and `apply`, defaulting to `full`, setting
   which source (`source` vs `lite_source`, per issue 357) is copied for newly-installed
   docs in that invocation.
2. Add a new verb, `harnez docs variant <name> <lite|full> [-d dir]`, that re-installs a
   single already-installed doc through the existing `installDoc`/`MergeManagedDoc` path
   — preserving any `harnez:stop`-delimited local section — and touches nothing else
   (`ref`, `local`, other docs, Makefile, AGENTS.md sections all untouched). This is
   the fast path for "I already have this doc installed, just swap its content."
3. Per `docs/CLIDesign.md`, `apply` stays global-only and `init` stays project-only; the
   new `docs variant` verb must accept a `-d <dir>` the same way other project-scoped
   verbs do, and must not blur the apply/init line.

## 3. Acceptance Criteria

- [x] `init --variant lite` and `apply --variant lite` install the lite source for any
      doc entry that has one, full source otherwise (no error for docs without a lite
      variant — silent fallback to full).
- [x] `harnez docs variant agentic-loop lite -d <repo>` swaps only that doc's installed
      content, preserves any local `harnez:stop` section, and reports success/failure.
- [x] Running `harnez status`/`harnez diff` immediately after a variant switch reports
      clean (confirms issue 358's marker-based drift detection is correctly wired here).
- [x] `go test ./...` and `scripts/smoke-test.sh` pass; `docs/CLIDesign.md`'s apply/init
      separation is unchanged (no project-local behavior added to `apply`).
- [x] Issue 231 closed as superseded by this mechanism, or explicitly re-scoped to
      whatever question (if any) remains after this ships — e.g. "which local models
      need lite by default" is a policy question distinct from the switching mechanism.
