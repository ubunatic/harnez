# 292 — `harnez index` reports "up to date" while `issues lint --cached` reports drift for the same repo, caused by unrelated untracked ticket files

**Status**: Closed — resolved: cached lint now names unrelated untracked ticket files and explains the staged-ticket-set mismatch; regression fixture added
**Priority**: P2 (Medium)
**Severity**: Bug (confusing failure, blocks unrelated commits)
**Category**: Issue Tracker Tooling
**Related**: [archive/054 filed issue-tracker files should be committed immediately](archive/054-commit-filed-issues-immediately.md)
(closed — a policy fix for a different angle: batching filed tickets before committing them; this
ticket is about the CLI's own two commands disagreeing about repo state, not about commit
discipline), `.git/hooks/pre-commit`'s `harnez issues lint --cached` invocation, feedback entry
`9539274c` (this session's first-pass note before it was promoted to a proper ticket)

---

## 1. Problem

Discovered live, 2026-09-09, while filing unrelated tickets (288, 291) in this repo: `git commit`
failed at the `pre-commit` hook stage with:

```
Error: issues lint: issues/README.md is generated index drift; run 'harnez index'
```

— immediately **after** running `harnez index -d .` in the same shell and having it print `issues/
README.md up to date`. Running the suggested fix does not fix it; the hook fails again on retry
with the identical message. This happened three separate times in one session.

**Root cause**: the two commands compute "the current ticket set" from different sources.

- `harnez index` (plain, no `--check`) regenerates `issues/README.md` from the full **working
  directory** contents of `issues/*.md` — including any untracked files that happen to be sitting
  there.
- The `pre-commit` hook runs `harnez issues lint --cached`, which (per its name) validates against
  the **staged/cached** git tree — i.e. only files that are actually part of the commit.

When an unrelated, untracked ticket file exists in `issues/` (a draft someone filed and hasn't
committed yet — concretely, `issues/285-*.md` and `issues/286-*.md` in this repo, both sitting
uncommitted since 2026-09-08), `harnez index`'s regenerated `issues/README.md` includes rows for
them, but the `--cached` lint's expected index does not (since they were never staged). Any commit
that touches `issues/README.md` at all — even one only adding/closing a completely unrelated ticket
— then fails, because the working-directory-generated `README.md` a caller staged doesn't match
what `--cached` computes as correct.

## 2. Impact

- The error message is actively misleading: it tells the caller to "run `harnez index`," but that
  is not the fix — the caller already ran it, and running it again reproduces the identical
  mismatch, because the tool that would fix a real index/source drift cannot fix a
  cached-vs-working-tree source disagreement.
- No indication of *which* file is causing the mismatch. A caller with no unrelated untracked
  drafts in `issues/` would never hit this and has no reason to suspect one; discovering the actual
  cause this session required manually diffing `git status --short` output against the regenerated
  `README.md`'s row list.
- Workaround used this session: temporarily `mv` the unrelated untracked ticket file(s) out of
  `issues/`, re-run `harnez index`, commit, then move them back. This is correct but entirely
  undiscoverable from the tool's own output.

## 3. Suggested Fix

Pick one (not mutually exclusive):

- Make `harnez issues lint --cached`'s error message name the specific file(s) causing the
  mismatch — a unified diff of expected-vs-actual `README.md` content (the same style `harnez
  index --check` already prints per its own `--help` text) would have made the actual cause
  obvious immediately instead of requiring manual investigation.
- Make `harnez index` (no `--check`) aware of untracked files in `issues/` and warn plainly ("N
  untracked ticket file(s) will not be reflected by `--cached` checks until committed") when it
  detects them, rather than silently regenerating a `README.md` that a subsequent `--cached` lint
  will then reject.
- Consider whether `issues lint --cached` should instead compute its expected index from the
  **union** of cached and untracked-but-present ticket files, rather than cached-only — i.e. match
  what a human actually sees in the working directory rather than only what's staged. This changes
  the semantics of "drift" itself, so weigh carefully against what `--cached` is meant to guarantee
  (a review of *this specific commit's* effect on the index) before choosing this over the two
  message/warning fixes above.

## 4. Verification

- Regression test: stage a ticket status change in a repo that also has an untracked, unrelated
  ticket file present; confirm the fix either (a) produces a clear, file-naming error/diff, or (b)
  succeeds by including the untracked file's row, per whichever fix direction is chosen.
- Live-verify against this repo's actual `issues/285`/`286` files (still uncommitted as of this
  writing) rather than only a synthetic fixture, since that's the real repro case that surfaced
  this.
