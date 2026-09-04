# 238 — `harnez init` doesn't gitignore `issues/README.md.lock` in managed repos

**Status**: In Progress — fresh sprint implementation
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[232-harnez-issues-verb-command-for-single-call-status-changes-with-index-sync-and-commit]]
(introduced `issues/README.md.lock`), `internal/index/index.go` (`lockReadme`/`unlockReadme`),
`.gitignore` (harnez's own repo root — has `/issues/README.md.lock` entry)

---

## 1. Problem & Motivation

Ticket 232 added an advisory-flock sidecar file, `issues/README.md.lock`, used by both
`harnez index` and `harnez issues <verb>` to guard concurrent writes to `issues/README.md`.
harnez's own repo has a `.gitignore` entry for it (`/issues/README.md.lock`), added as a
follow-up during 232's work — but nothing in `harnez init` (or `harnez apply`) propagates an
equivalent `.gitignore` entry into other managed repos.

Observed tonight: onboarding `trafficsim` and `.workspace` via `harnez init`, then running
`harnez index -d <repo>` in each (see ticket 237 for the more serious finding from the same
run), left an untracked `issues/README.md.lock` file behind in both repos' working trees. Every
other project that runs `harnez index` or `harnez issues <verb>` will accumulate this same
untracked-file noise unless it happens to already ignore `*.lock` or the maintainer notices and
adds the entry by hand, as harnez's own repo did.

## 2. Scope

- Decide where `.gitignore` management belongs for harnez-managed repos: `harnez init` writing/
  merging a `.gitignore` entry, `harnez apply`, or a check surfaced via `harnez status`
  (existing linter, per `docs/practices/IssueTracking.md` §4.1) flagging the untracked lock file
  if `issues/` is git-tracked and the entry is missing.
- Search codebase confirms no existing `.gitignore`-writing logic anywhere in Go source (`grep
  -rln gitignore --include=*.go` returned nothing) — this would be new functionality, not a
  regression in an existing mechanism.

## 3. Open Questions

- Should `harnez init` own `.gitignore` edits at all, or is a `harnez status` warning
  ("`issues/README.md.lock` is untracked and unignored") sufficient and less invasive, given
  `init`/`apply` are meant to stay narrowly scoped per `docs/CLIDesign.md`?
- If `harnez init` does own it: additive-merge into an existing `.gitignore` (don't clobber
  unrelated entries), matching the caution this same investigation raised for `issues/README.md`
  itself in ticket 237.

## 4. Acceptance Criteria

- [ ] A fresh `harnez init` (or the first `harnez index`/`harnez issues <verb>` run) leaves no
  untracked, unignored `issues/README.md.lock` behind in a managed repo — either via an added
  `.gitignore` entry or a surfaced `harnez status` warning, per whichever design is chosen above.
