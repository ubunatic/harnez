# 1072 — plain git commit fails after harnez init: pre-commit issues lint errors when issues/ does not exist

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Major (every commit in a freshly initialised repo is blocked)
**Category**: Bug
**Related**: `internal/claude/init.go` (`issuesHookBlock`), `internal/issues/issues.go` (`scan issue files`)

---

/goal Make a plain `git commit` succeed straight after `harnez init` in a repo with the Harnez tracker and no `issues/` directory, with a regression test, or stop and report when blocked on a user decision or denied permission. On a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation
After `harnez init` in a new repo with `issues.tracker: harnez`, the first `git commit` fails, and so does every later one:

```text
Error: issues lint: scan issue files: stat .: no such file or directory
```

Init installs the pre-commit hook that runs `harnez issues lint --cached`, but does not create `issues/`. The message does not name the missing directory or a fix, so users are pushed towards `--no-verify`. Found while making the first commit in the `explorers` repo.

## 2. Technical Specification / Findings
Reproduced with harnez 0.2.10+4393:

1. `git init` a new directory, add a `.harnez/repo.yaml` with `issues.tracker: harnez`, run `harnez init -y --issues-git`.
2. `git add -A` and `git commit`: exit 1 with the error above. A second commit, after a `--no-verify` baseline, fails the same way.

Related behaviour:

- `harnez index` prints `no enabled issue or studies indexes ... nothing to do` while `issues/` is missing, so the obvious repair is a no-op.
- `mkdir issues` makes `harnez index` generate the three index files, but `issues lint --cached` still fails until they are staged. In `explorers`, the commit then passed.
- `harnez init -y` without a recorded tracker chooses `tracker: none` and installs no hook, so that path is not affected. How `explorers` got its recorded tracker was not traced; the wizard path has not been tested.

## 3. Implementation & Verification Plan
Re-check the current code first. Options to decide between: init creates and indexes `issues/` when it records the Harnez tracker, or lint treats a missing `issues/` as an empty tracker. Either way, `harnez index` should not report "nothing to do" for a repo whose tracker is enabled.

Verify with a regression test covering the steps in section 2: the first commit succeeds with the hook active and without `--no-verify`.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-1072-audit | 14066435479314044408 | github #166 (closed) | discarded: claims "still valid", but the fix is in |

Checked 2026-11-11: in an empty git repo, `harnez issues lint --cached` prints "issues lint: no issues/ directory
yet, nothing to lint" and exits 0, so the closure stands. The report claimed it ran `harnez init`, which Jules
cannot do.
