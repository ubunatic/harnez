# 1082 — release: push fails when a forge push mirror updates another remote first

**Status**: Closed — Remote that already holds the pushed branch and tag is accepted (internal/release/push.go, ls-remote check after a failed push); TestReleasePushAllRemotes race-success fails without fix, race-fail still errors. make test-q1 exit 0. From Jules PR 143 (closed, cherry-picked).
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: uman issue 039

---

## 1. Problem & Motivation
`harnez release` stops before publishing when a repo has two forge remotes and the first one mirrors
to the second. The release is left half done: tag pushed everywhere, no forge release created.

## 2. Technical Specification / Findings
Seen in a real emojig v0.2.3 release on 2026-11-01. Remotes were `origin` (Codeberg) and `mirror`
(GitHub). The push to `origin` succeeded, then the push to `mirror` failed:

```
! [remote rejected] main -> main (cannot lock ref 'refs/heads/main': is at b221fb5... but expected 3899f3e...)
! [remote rejected] v0.2.3 -> v0.2.3 (failed)
```

Afterwards both remotes had the correct `main` and tag. This points to the forge's push mirror
updating GitHub during harnez's own push, though the mirror configuration was not inspected.
`internal/release/push.go` pushes each remote in turn after a dry-run preflight, so the preflight
cannot catch this race.

Update from the uman session: the owner removed emojig's mirror automation, which works around the
race for that repo, though no retried release has confirmed it yet. Other repos with a second forge
remote can still hit it.

## 3. Implementation & Verification Plan
When a remote push fails, re-read that remote's refs with `git ls-remote`. If the branch and tag
already match the local commit and tag object, log it and continue instead of failing. Any other
state still fails.

Verify with a unit test using local bare repos: update the second remote during the sequence, and
assert that the release continues, while a differing ref still fails.

## Audit 2026-11-03

Jules audit run `jules-audit-1082` (session 10011213719013868384, against b0dcb501) failed: it ended asking whether to ignore a `make check` gofmt failure in `internal/release/pubkeys_test.go` and opened no PR, so no report. Per the one-shot rule it was not answered. The gofmt issue is fixed; re-audit if this ticket still matters.

## Audit 2026-11-04

Jules re-audit (session 5341120481487332044, against 70da579e, `reports/jules/1082.md`): still valid, 4 of 4
references checked. The real push loop (`internal/release/push.go:79-84`) fails on the first rejected
push with no `git ls-remote` re-check; `internal/release` has no `ls-remote` call. The fan-out skip
(`selectPushRemotes`/`coveredByKept`, same file) is a different feature and does not cover this race.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-1082 | 2739167954748901484 | github #93 (closed) | done: still valid, 5/5 refs hold, report kept |
