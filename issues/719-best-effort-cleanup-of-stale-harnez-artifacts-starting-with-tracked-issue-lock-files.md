# 719 — Best-effort cleanup of stale harnez artifacts, starting with tracked issue lock files

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[279-avoid-persistent-issues-readme-lock-sidecar-in-working-trees]], [[238-harnez-init-doesn-t-gitignore-issues-readme-md-lock-in-managed-repos]], issue 672 (`init.go` excludes the locks), `internal/claude/init.go`

---

## 1. Goal

/goal When harnez works in a repo and sees old files of its own that are likely stale, it cleans
them up best-effort (or tells the user how, when removal is unsafe), so leftovers do not linger in
working trees or git history; or stop and report when blocked on a safety question (issue 279) or
an owner decision.

## 2. Problem (owner, 2026-10-06)

`issues/README.md.lock` and `issues/.reserve.lock` are **tracked in git** in 7 repos under
`~/projects`: goto, kernel, loom-games, neus, research, settings, termaid (loom-games' dated from
issue 001, Sep 27). `harnez init` now adds both to `.git/info/exclude` (672), but exclusion does not
untrack files that are already committed, so they stay and look like stale state. loom-games was
cleaned by hand in 0e81cd4; the other 6 on 2026-10-06 (goto 23d8aba, kernel d9000b0, neus
13f0871, research 38f58e3, settings 931409e, termaid f1fe7ab: `git rm --cached` plus missing
`.git/info/exclude` entries). The automatic check is still open.

## 3. Scope & Constraints

- **Tracked lock files:** when a harnez command runs in a repo and finds these files tracked, untrack
  them (`git rm --cached`) or print the exact command; never commit on the user's behalf unless the
  command already commits (e.g. `harnez issues … --commit`).
- **Other likely-stale artifacts:** list candidates first (old lock sidecars, leftover temp or
  reserve files from interrupted runs) and decide per kind; record the list here.
- **Respect 279:** the lock sidecar is a stable `flock` inode; deleting it can let two processes
  hold independent locks. Do not delete live lock targets by age; untracking is safe, deletion only
  if 279 relocates the lock or the command holds every relevant lock.
- Best-effort: a cleanup failure never fails the user's command.

## 4. Verification

- In a repo with tracked lock files, the next harnez command leaves them untracked (or prints the
  command); a second run is a no-op.
- Concurrency test from 279 still passes.
- The 6 remaining repos are clean afterwards (done by hand, see §2).
