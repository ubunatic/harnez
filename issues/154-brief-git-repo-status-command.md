# 154 — `harnez repo-status`: brief, quiet-by-default git repo state summary

**Status**: Closed — resolved in 9f16839
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Tooling
**Related**: `cmd/harnez/distill.go`, [[148-harnez-index-command-for-issues-docs-studies]]

## Problem

The user runs plain `git status` frequently inside a Claude Code (or other
agent) session to check "is everything clean, is there anything pending,
are we ahead of origin." When run through the agent, the agent reads the
full `git status` output and then narrates it back in prose — even when the
honest answer is "nothing to report" — which costs real output tokens on
every single check, most of which convey zero new information.

No existing harnez command covers this. `harnez status` (`cmd/harnez/main.go`)
reports config/managed-doc apply state, not git working-tree state.
`harnez distill -- git status` (see `cmd/harnez/distill.go`) strips generic
noise (ANSI, repeated lines) from arbitrary command output, but has no
git-specific semantics — it wouldn't know how to collapse "clean tree, N
commits ahead, no other signal" into a single confirmatory line, or when a
diffstat or file list is actually worth surfacing.

## Requirements (from the user's own framing)

- A single command that summarizes git repo state in **only a few lines**
  by default.
- When everything is fine — nothing to commit, nothing staged/unstaged,
  working tree clean, nothing else notable — output should be **a few
  words/one line** confirming that ("clean", "up to date", "nothing
  pending"), not a full `git status`-shaped report.
- Only expand to more detail (diffstat, file list, ahead/behind counts,
  conflict state, etc.) when there is something **big, pending, or broken**
  to report: uncommitted changes of meaningful size, unstaged/staged files,
  merge conflicts, detached HEAD, a diverged/behind-and-ahead state, etc.
- The bar for "big" vs. "brief" needs a concrete threshold (e.g. file count,
  line-count diffstat size, or simply "any changed/untracked/staged file at
  all is not brief") — pick one and document it; don't leave it fuzzy.
- Being ahead of `origin/main` by N commits (the common, expected state in
  this solo/no-PR-workflow repo per this project's own `CLAUDE.md`) should
  count as normal/quiet-worthy on its own, not trigger the verbose path by
  itself — only actual pending/broken state should.

## Scope (proposed, not settled — revisit at implementation time)

- New subcommand, e.g. `harnez repo-status` or `harnez git-status` (name
  TBD — check it doesn't collide with `harnez status`'s existing meaning
  before picking one).
- Wraps `git status --porcelain=v2 --branch` (or equivalent) for
  machine-parseable input rather than parsing human-readable `git status`
  text.
- Quiet path: one line, e.g. `clean, up to date` / `clean, 3 ahead of
  origin/main`.
- Verbose path: short structured summary (not the full raw `git status`
  dump) — counts by category (staged/unstaged/untracked), branch
  divergence, conflict markers if present — still shorter than raw
  `git status` where possible.
- Consider whether this belongs as a genuinely new command, or as a
  `--porcelain`/`--brief` mode bolted onto an existing one — evaluate
  against `harnez status` and `harnez distill` before committing to a new
  top-level command (this repo has been actively reassessing command-tree
  placement, see [[153]]).

## Acceptance Criteria

- [x] Command name and placement decided (new command vs. flag on an
      existing one), with the alternatives above considered and the
      decision recorded.

      **Decision**: new top-level command `harnez repo-status`
      (`cmd/harnez/repostatus.go`), not a flag on an existing command.
      `harnez status` (`cmd/harnez/main.go`) already has an established,
      unrelated meaning — config/managed-doc apply state, not git working-
      tree state — so overloading it would be confusing. `harnez distill --
      git status` (`cmd/harnez/distill.go`) strips generic output noise but
      has no git-specific semantics (it can't tell "clean, 3 ahead" apart
      from a genuinely noteworthy diff, or classify quiet vs. verbose).
      Neither existing command is a natural home, so `repo-status` is its
      own subcommand. Ticket 153 (command-tree placement assessment) was
      explicitly kept out of scope for this change, per its own deferred
      status — this decision only concerns 154's own command, not a
      broader tree reorganization.
- [x] Clean-repo case: output is one short line, no full status dump.
- [x] Dirty/pending/broken case: output includes enough specifics to act on
      (what's staged/unstaged/untracked, conflicts, divergence) without
      requiring a follow-up `git status` call.
- [x] "Ahead of origin by N" alone (no other changes) is treated as quiet,
      not verbose.
- [x] `go test ./...` passes; a test exists for both the quiet and verbose
      paths using a real temp git repo (not just parsed-string fixtures),
      matching this project's convention of exercising real state where
      practical.

## Implementation Notes

- `internal/gitstatus` parses `git status --porcelain=v2 --branch` into a
  `Status` struct (branch, upstream, ahead/behind, staged/unstaged/
  untracked/conflict file lists) and exposes `Status.Quiet()`.
- **Quiet/verbose threshold (concrete, per AC)**: `Quiet()` is true iff
  ALL of: no staged files, no unstaged files, no untracked files, no
  conflicted files, HEAD not detached, AND behind-count is 0. Ahead-count
  is excluded from the check entirely — being ahead of the upstream by any
  N, with nothing else pending, is quiet. Being behind by any amount
  (including a full ahead-and-behind divergence) is verbose: local history
  not being a superset of upstream's is worth surfacing, unlike the
  expected-normal "unpushed local commits" state in this solo/no-PR-
  workflow repo.
- Quiet path prints one line, e.g. `clean, main, up to date with
  origin/main` or `clean, main, 3 ahead of origin/main`.
- Verbose path prints branch/divergence, a `staged=N unstaged=N
  untracked=N conflicts=N` count line, and per-category file lists (sorted,
  capped at 10 paths with a "+N more" suffix) — structured, not a raw
  `git status` dump.
- Tests: `internal/gitstatus/gitstatus_test.go` (parser unit tests plus
  real-temp-git-repo tests, including a bare-remote ahead/behind scenario)
  and `cmd/harnez/repostatus_test.go` (command-level tests against real
  temp repos for the quiet, verbose, and ahead-only-is-quiet cases).
- Manually verified against this repo (quiet before starting; verbose
  while mid-change) and a scratch temp repo (clean → quiet; edited file →
  verbose) — see session transcript.
- `make install` run; `go build ./...` and `go test ./...` pass repo-wide.

## Notes

Low priority — this is a token/ergonomics convenience, not a correctness
issue. Ticket 153's command-tree placement assessment was intentionally
NOT bundled into this change; only the placement decision needed for 154
itself (above) was made here.
