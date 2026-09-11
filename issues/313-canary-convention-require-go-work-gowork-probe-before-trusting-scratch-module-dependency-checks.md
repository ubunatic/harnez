# 313 — Canary convention: require go.work/GOWORK probe before trusting scratch-module dependency checks

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: [issues/310](310-check-for-ambient-enclosing-go-work-in-harnez-status-and-lint.md), [issues/287](287-go-work-ambient-workspace-file-breaks-unrelated-tooling-in-sibling-repos-general-fix-needed.md), [issues/309](309-manage-go-workspace-via-go-work-example-and-symlink-with-init-gowork.md), [docs/Canary.md](../docs/Canary.md)

---

## 1. Problem & Motivation

An ambient or project-local `go.work` file silently redirects Go module
resolution to sibling workspace modules (see [[287]], [[309]], [[310]]). A
canary probe that checks "does this dependency/version/API exist in the
target repo's module graph" can pass or fail for the wrong reason if it
doesn't first confirm whether a workspace is in play — the probe may be
observing a sibling module's resolution instead of the target repo's own
`go.mod`.

[[310]] adds ambient-`go.work` detection to `harnez status`/`harnez lint`
as a runtime diagnostic. This ticket is narrower and doc-only: the
canary-writing *convention* itself (`docs/Canary.md`) should tell agents
to check workspace state before trusting a scratch-module dependency
canary, independent of whether `harnez status` is run in that session.

## 2. Scope

- Add a line item to `docs/Canary.md` (in the mechanism-isolation /
  "before you build" guidance) stating that any canary probing Go module
  dependencies must first check for an active workspace, e.g.:
  ```sh
  test -f go.work || go env GOWORK
  ```
  and note in the canary's output whether a workspace was active, so the
  probe result is interpreted correctly.
- Consider extending `docs/templates/Makefile` (or the relevant canary/
  check target scaffolding) with a small check target or guard that
  surfaces this same probe, so `init`-scaffolded projects get it for
  free. Evaluate feasibility as part of this ticket rather than
  committing to it upfront — this is a "consider," not a hard
  requirement.
- Out of scope: `harnez status`/`harnez lint` runtime detection — that's
  [[310]].

## 3. Acceptance Criteria

- `docs/Canary.md` has a line item documenting the `go.work`/`GOWORK`
  probe requirement for Go-dependency canaries.
- A decision (yes/no + rationale) is recorded on whether the Makefile
  template gets a corresponding check target; if yes, it's implemented
  and verified with `make -C docs/templates` or the project's own
  `make check`/`make smoke` as applicable.

## 4. Verification

- Doc-only change: review that the new line item reads clearly in
  context and doesn't duplicate [[310]]'s runtime-detection framing.
- If a Makefile template check is added, run it against a repo with and
  without an ambient `go.work` to confirm it surfaces the expected
  signal.
