# 287 — Ambient go.work breaks unrelated sibling repositories; adopt a general fix

**Status**: Closed — resolved in b1e131a
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Infrastructure
**Related**: [issues/284](284-harnez-release-build-step-defaults-to-gowork-off-with-allow-workspace-override.md) (first instance, fixed narrowly for `harnez release`'s own build step), [docs/practices/GoRelease.md §4 "go.work and Local Co-Development"](../docs/practices/GoRelease.md)

---

## 1. Problem & Motivation

`~/projects/go.work` (`use ./harnez ./voxi`) exists for legitimate
cross-repo co-development — harnez's `internal/usage/mic.go` imports
`ubunatic.com/voxi/audiolevel` (see issue 284). Go auto-detects this file
by walking up from the CWD, so it silently applies to *any* `go
build`/`go run`/`go list`/etc. invoked from a working directory nested
anywhere under `~/projects` — not just inside `harnez` or `voxi`.

Issue 284 found and fixed the first instance: `harnez release`'s own
build step was silently affected, forced `GOWORK=off` by default with a
`--allow-workspace` opt-back-in.

**Second instance, found live 2026-09-08, in a third, unrelated repo —
`~/projects/ubunatic.com`:** running `uman website sync voxi` failed:

```
directory gopkg is contained in a module that is not one of the
workspace modules listed in go.work. You can add the module to the
workspace using:
	go work use .
make: *** [Makefile:21: ingest-gopkg] Error 1
```

Root cause: `ubunatic.com/Makefile`'s `ingest-gopkg` target runs `cd
scripts && go run ./gopkg ingest $(PKG)` — `scripts/` is its own Go
module (`ubunatic.com/scripts`), nested under
`~/projects/ubunatic.com/scripts`, itself nested under `~/projects`.
Because `~/projects/go.work` only lists `./harnez` and `./voxi`, Go
refused to run a module that isn't a workspace member — even though this
has nothing to do with the harnez/voxi cross-dev workspace at all. It's
collateral damage on a completely unrelated repo's unrelated tooling.

Worked around live (uncommitted, in `ubunatic.com`, out of scope for
this ticket) by adding `export GOWORK := off` near the top of
`ubunatic.com/Makefile` — same fix *pattern* as issue 284, hand-applied
to yet another repo's build tooling.

This is not a one-off: `~/projects/.uman.toml` lists ~20 sibling
projects (books, cati, mdview, pdf-doctor, psync, spriteview,
trafficsim, uman, wayreel, webman, etc.), any of which could hit the
identical failure the moment a `go` command runs from inside it while
`~/projects/go.work` exists and doesn't list it. Patching each sibling's
Makefile by hand as it's discovered does not scale and leaves the
blast radius live for every repo not yet hit.

## 2. Scope — Review Later, Decide General Fix

This ticket is filed to review later and pick a *general* solution, not
to implement one now. Directions to evaluate (none pre-decided; note
which are drawn from existing project docs vs. new suggestions from this
ticket's filing):

1. **Should `~/projects/go.work` exist ambiently/persistently at all?**
   Given its blast radius on unrelated tooling across every sibling
   repo, consider creating/removing it on demand only for an active
   harnez+voxi co-dev session (e.g. a documented `go work use`/`go work
   edit -dropuse` dance, or a wrapper script) instead of it sitting
   permanently in `~/projects`. (New suggestion, not drawn from existing
   docs.)
2. **Is there a Go-level mechanism to scope a workspace file's effect
   more narrowly** than "any CWD nested anywhere below it" — e.g. does
   `GOWORK` support a per-invocation opt-in instead of ambient opt-out,
   or is `GOWORK=off`-by-default-then-opt-in feasible for interactive
   shell use (as opposed to build scripts, which is what issue 284
   already solved)? (New suggestion; needs a canary check against real
   `go` tooling docs/behavior before treating any answer as verified,
   per this project's canary-first-development practice.)
3. **Should `uman` itself defensively set `GOWORK=off` for every
   subprocess it shells out to** (build hooks, sync hooks, etc.) by
   default — mirroring what `harnez release`'s build runner already does
   (issue 284) — so individual sibling repos' Makefiles don't each need
   to know about this ambient workspace file? This would be a single fix
   point (in `uman`, source likely at `~/projects/uman`) instead of N
   per-repo patches. (New suggestion, structurally analogous to issue
   284's fix but at the orchestrator level instead of per-repo.)
4. **Where should this be documented once decided** —
   `docs/practices/GoRelease.md` §4 "go.work and Local Co-Development"
   (added for issue 284) is currently scoped to release tooling
   specifically; this blast-radius risk is broader than release tooling
   (it hit a `uman`/website-sync workflow, not a release). Decide whether
   §4 gets broadened, or a new/renamed doc is warranted.

## 3.1 Proposed `harnez init` policy

As a project-wide default, make `harnez init` inspect the Go workspace
situation before writing any project-local workspace file. The proposed
decision rule is:

1. If the target repository already has a local `go.work`, leave it alone.
2. If it has no local `go.work`, discover whether Go sees a parent
   `go.work`, and whether the target project/module is listed in that parent
   workspace.
3. If a parent workspace exists but does not include the target project,
   create a minimal local `go.work` for the target's Go modules, such as
   `use ./scripts`, when the repository has a Go module that needs the
   isolation. This makes the repository's normal `go` commands resolve from
   its own declared modules rather than inheriting an unrelated parent
   workspace.
4. If there is no Go module, no parent workspace hazard, or no evidence that
   a local workspace is useful, do not create `go.work` merely because
   `harnez init` ran.

The implementation should run a small real `go` probe rather than infer the
answer from directory names alone. Candidate probes include `go env GOWORK`
from the target repository and, for each relevant module, `go list -m` or an
equivalent command that confirms whether the discovered parent workspace
contains that module. The probe must distinguish a local workspace from an
ambient parent workspace and must handle `GOWORK=off` explicitly.

The guiding rule is least surprise: `harnez init` should make the best
choice for the repository's actual shape, but should not add files
prematurely. A generated local `go.work` is justified only by a demonstrated
ambient-workspace hazard and a detected Go module; otherwise initialization
must remain file-preserving. The behavior should be reported in the init
summary and support a dry-run or explainable decision path so users can see
why a workspace file was or was not created.

## 3.2 Implemented policy and verification

`harnez init` now discovers all repository-local Go modules, asks Go which
workspace is active with `go env GOWORK`, and reads Go's parsed workspace
membership with `go work edit -json`. It preserves an existing local `go.work`
and skips creation when there is no module, no enclosing workspace, an explicit
`GOWORK` selection, or complete parent-workspace membership.

When an automatically discovered parent workspace omits at least one local
module, init runs `go work init` with parent workspace discovery disabled for
that creation command and includes every local module. Init prints the selected
action and reason in every case. The shared policy and explicit opt-in command
for intentional cross-repository work are documented in `docs/lang/Go.md`.

Focused tests cover an unrelated parent workspace, a project already listed in
the parent, no parent workspace, no Go module, an existing local workspace,
`GOWORK=off`, and a multi-module repository whose parent lists only one module.
A live installed-binary canary under `/home/uwe/projects/go.work` reproduced the
pre-init `go list` failure, created `use ./scripts`, and passed the same Go probe
after init. The original mutating `uman website sync voxi` workflow was not
rerun; the canary exercises its underlying Go workspace failure without
touching that separate repository.

## 3. Non-Goals (for now)

- No implementation is included in this filing commit; investigation and
  implementation belong to the follow-up work tracked here.
- Not touching `~/projects/ubunatic.com` (already worked around ad hoc
  there this session; that fix is uncommitted and out of scope here).
- Not deciding here whether the harnez+voxi `go.work` should exist at
  all — recorded above as an open question for whoever picks this up,
  not as this ticket's own opinion.

## 4. Acceptance Criteria

- [x] Reproduce and document the ambient-workspace failure with a small
      canary from a Go module below `~/projects` that is not listed in
      `~/projects/go.work`.
- [x] Evaluate the workspace-lifecycle, Go-environment, and orchestrator-level
      options above against normal harnez+voxi co-development and unrelated
      sibling-repository commands.
- [x] Record the chosen general policy and why it is preferred, including how
      developers deliberately opt into the harnez+voxi workspace when needed.
- [x] Implement the chosen fix at the narrowest shared control point that
      protects unrelated sibling repositories without requiring ad hoc edits
      in every repository.
- [x] Verify the underlying `uman website sync voxi` failure path with an
      equivalent live sibling-repository canary, plus an intentional
      harnez+voxi workspace workflow. The mutating `uman` command itself was
      intentionally not rerun.
- [x] Update the appropriate shared Go guidance so future repositories and
      automation inherit the policy.
- [x] Add `harnez init` probes for: no local workspace with an unrelated
      parent workspace; a project already listed in the parent workspace; no
      parent workspace; and a repository with no Go module.
- [x] Verify that `harnez init` creates a minimal local `go.work` only for
      the first hazard case, preserves existing local files, and reports the
      decision without creating files in the other cases.
