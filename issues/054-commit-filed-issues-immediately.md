# 054 — Filed issue-tracker files should be committed immediately, not batched

**Status**: Open
**Category**: Agentic Ergonomics / Harness Behavior
**Related**: [043](043-never-blindly-revert-commit-stale-work-first.md), [044](044-git-md-proactive-commit-vs-harness-ask-first.md), [046](046-commit-checkpoint-recurred-after-044-filed.md)

---

## 1. Problem & Motivation

[044](044-git-md-proactive-commit-vs-harness-ask-first.md) already
documents the broader tension between `docs/Git.md`'s "commit
proactively" guidance and a harness's ask-first default for general code/
feature work. [046](046-commit-checkpoint-recurred-after-044-filed.md)
found the exact same gap recurring in the same session that filed 044.

This ticket is narrower and specifically about **issue-tracker files**
(`issues/NNN-*.md` plus the index they're listed in), not general code:

- They're append-only, low-risk, metadata-not-behavior content — filing
  one changes nothing about how any tool runs.
- Their entire value is as a durable record other sessions (or the same
  session after compaction) can find. An uncommitted issue file is not
  durable — it's one `git checkout --`, one bad `stash`, one crashed
  session away from silently vanishing, which defeats the tracker's
  purpose more completely than an uncommitted code change does (code
  changes are usually visible in `git status`/`git diff` and get noticed;
  a lost issue file is invisible unless someone remembers it existed).
- **Self-referential proof this matters**: this very ticket and
  [053](053-stale-lsp-diagnostics-noise-detect-and-toggle.md) were both
  filed in the same session, sitting uncommitted in `harnez`'s own working
  tree, until the user explicitly asked for them to be committed — the
  harness's general ask-first default applied to them exactly as it would
  to a code change, with no special case for "this is just a tracker
  entry." That's the gap.

## 2. What's being requested

Distinct from 044's broader ask (which is about code/feature commits and
proposes documenting the ask-first fallback explicitly): this ticket
proposes issue-tracker files specifically get a **lower bar**, not just a
documented fallback —

1. `docs/practices/IssueTracking.md` (or wherever this repo's issue-filing
   convention lives) should say explicitly: once an issue file is written
   and its index updated, commit it immediately, in its own small commit,
   separate from any code change — don't batch it with unrelated work, and
   don't wait for a broader "should we commit now" checkpoint.
2. For harnesses with an ask-first commit default (see 044): the
   recommendation is to treat "I just filed an issue" as a standing,
   pre-authorized trigger to *ask* "commit this issue file now?" rather
   than folding it into the same judgment call as "should we commit this
   feature work" — the two have very different risk profiles (a tracker
   entry vs. a behavior change) and shouldn't share one commit-cadence
   decision.
3. Whether this can be pre-authorized more durably (e.g. a project-level
   permission rule scoped narrowly to `git commit` when the only staged
   changes are under `issues/`) is worth exploring, but is a harness/
   settings question, not something this ticket resolves on its own.

## 3. Explicit non-goal

Not proposing issue files bypass version control review entirely, or that
they get committed without the user ever being asked in a harness that
requires it — just that the *ask* should happen right after filing, every
time, rather than being deferred to whenever a broader commit
conversation happens to occur (which is what let 046's recurrence, and
this ticket's own filing, both happen).
