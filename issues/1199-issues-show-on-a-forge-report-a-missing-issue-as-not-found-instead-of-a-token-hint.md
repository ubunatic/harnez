# 1199 — issues show on a forge: report a missing issue as not found instead of a token hint

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1116 (forge show/find), 1118 (forge token)

---

/goal `harnez issues show <n>` on a forge tracker prints "issue #<n> not found on <host>/<owner>/<repo>" when the
repo resolves but the issue does not, and keeps the token hint only when the repo itself is unreachable. Stop and
report when blocked on a user decision; on a loop or no progress, clear the goal and write a failure report here.

## Problem

Reported by the taia session (2026-11-09): `harnez issues show -d . 99999` on the public repo
codeberg.org/harnez/taia prints "HTTP 404 ... (a private repo needs a token: run harnez forge token import ...)".
The hint misleads: the repo is public, only the issue is missing.

## Want

- On 404, probe the repo endpoint once; repo OK -> "not found"; repo 404 -> current token hint.
- Unit test with a mock forge for both cases.

## Notes

- taia also asked whether the "no local issues/ dir: tickets live on ..." notice pollutes stdout. Checked: it is
  written to stderr (`cmd/harnez/issues_forge.go:48`), so no change needed.
