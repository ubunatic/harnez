# 246 — Add /commit and /publish Skills for Staged Commit Ownership and Multi-Project Release/Publish Workflows

**Status**: Open — filed via /issue
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: `commands/harnez-sync.md` (closest existing precedent for a CLI-driving orchestration
skill that dispatches a fresh subagent and stays non-interactive), `commands/fresh-sprint.md`
(main-agent-vs-subagent dispatch pattern this ticket's "who commits" split needs to reuse), `../
smarthome/Makefile` (`release`, `release-preflight`, `release-build`, `website-privacy` targets —
concrete example of a project's own `make release`), `.uman.toml` (cross-project publish/sync
mechanism), `docs/Git.md` (conventional commits, work on default branch, don't push unless asked)

---

## 1. Problem & Motivation

Two related but distinct workflow gaps, requested together:

1. No `/commit` skill exists to ask an agent to commit all pending changes, with an explicit rule
   for **who** does the committing when a subagent was involved.
2. No `/publish` skill exists to ask an agent to publish/release the current project's product,
   where "publish" varies by project shape (CLI tool vs. website vs. app) and can span multiple
   repos in a single logical release.

## 2. `/commit` — Requested Behavior

Ask the agent to commit all pending changes. The **ownership split** is the concrete, load-bearing
part of the request (not just "run `git commit`"):

- **Main (host) agent commits** when there are many different, unrelated changes accumulated —
  e.g. post-subagent fixes layered on top of other work, or changes spanning multiple concerns that
  don't cleanly belong to one subagent's scope.
- **The original subagent commits** when there was one dispatched subagent and the pending changes
  are mostly/only related to that subagent's own work — i.e. commit ownership should follow whoever
  has the most direct, complete picture of *why* the change was made, not default to the host by
  convention.

This mirrors (and should probably reuse or reference, not duplicate) the fresh-sprint pattern's
existing Dev-Worker-vs-Host framing (`docs/AgenticLoop.md` §5 Role Taxonomy) rather than inventing a
separate policy — worth checking during implementation whether this is really a new skill or
whether it's better framed as "make committing an explicit closing step every fresh-sprint/sprint
dispatch already does" plus a standalone `/commit` for ad hoc use outside those flows.

**Open questions** (exploratory — not yet resolved):
- How does `/commit` decide "many different changes" vs. "mostly one subagent's work" concretely —
  by diffing file paths against a subagent's reported touched-files list? By asking the user when
  ambiguous? Not specified by the request; needs a design pass.
- Does `/commit` ever get invoked when no subagent was involved at all (host did everything
  directly)? Presumably yes — that's the simple case and should just work like a normal `git commit`
  following this repo's existing commit conventions (`docs/Git.md`).
- Should `/commit` refuse to commit files it can't attribute a clear reason for, rather than
  guessing at a commit message? Given this repo's own git-safety norms (never `git add -A`
  blindly, review staged content before committing), `/commit` should inherit those safety rules,
  not bypass them for speed.

## 3. `/publish` — Requested Behavior

Ask an agent to publish/release the current project, where the mechanism depends on project shape:

- **CLI/tool projects**: `make release` or `harnez release`, whichever the project actually defines
  — `/publish` should detect which is present rather than hardcoding one (see `../smarthome`'s own
  `release`/`release-preflight`/`release-build`/`release-sign` Make target chain as a concrete
  example of a project-owned release pipeline `/publish` would need to drive, not reimplement).
- **Website-shaped projects/subprojects**: publishing means the site actually goes live *and* its
  source is pushed — both steps, not just a local build. Concretely, per the user's own example
  (`../smarthome`): build the release artifact (APK), update the project's website ideally via a
  spec-driven `make website` step so version links etc. are generated from source-of-truth data
  rather than an agent hand-editing HTML, then `uman website sync <project>` to publish it, then
  `make sync` (or the equivalent) in `../ubunatic.com` (the main publishing repo, per
  `CLAUDE.md`'s "Workspace (uman)" section) so the project's subpage actually goes live on the
  real published site — not just built locally.

**Open questions** (exploratory — not yet resolved):
- Should `/publish` be generic-and-detect-shape (probe for `make release`, `harnez release`,
  `website/` dir, etc. and act accordingly), or should each project declare its own publish recipe
  explicitly (e.g. a `docs/Publish.md` or a `.uman.toml` entry) that `/publish` reads rather than
  infers? The generic-detection approach risks silently doing the wrong multi-step sequence for a
  project shape nobody anticipated; an explicit per-project declaration is safer but requires
  authoring one per project before `/publish` works there.
- `uman --help` should be probed before building any cross-repo mechanism (per `CLAUDE.md`'s
  existing "Workspace (uman)" note, including its `uman issue 015` caveat that custom uman
  subcommands execute even under `--help` — read `.uman.toml` directly instead of probing them).
- This is inherently a **hard-to-reverse, externally-visible action** (pushes code, makes a website
  go live) — per this harness's own "Executing actions with care" policy, `/publish` must not run
  end-to-end autonomously without a confirmation step before the actual push/sync/go-live actions,
  even if earlier steps (build, `make website`) are safe to run non-interactively.

## 4. Implementation & Verification Plan

### Milestone 1: `/publish` Skill (Implemented in this sprint)
- Created `commands/publish.md` with 5-stage publishing pipeline:
  1. Pre-flight verification & cleanliness checks (`git status`, `make check`, `make test`, artifact dry-runs).
  2. Media & artifact verification gate.
  3. Mandatory user confirmation gate before any irreversible external actions (`git push`, forge release, `ubunatic.com` live sync).
  4. Live execution: `harnez release` or `make release`, `uman website sync <project>`, `ubunatic.com` build & deploy via `make sync`.
  5. Post-publish 3-state grounding & live HTTP 200 liveness probe (`@docs/practices/DeploymentTransparency.md`, `@docs/practices/GoRelease.md` §6).
- Registered `publish` under `commands:` and `skills:` in `config.yaml`.
- Verified roundtrip generation in `internal/claude/claudeskills_test.go` and verified with `scripts/lint.sh` and `make test`.

### Milestone 2: `/commit` Skill (Pending / Follow-up)
- Follow-up work to define `/commit` for staged commit ownership split (main host agent vs original subagent).
