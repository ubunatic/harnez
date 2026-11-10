# 1207 — harnez issues comment: add a comment to a forge issue via fj, safely

**Status**: Closed — harnez issues comment -F file adds a comment via fj after an API preflight (PR refused, closed needs --closed, --forge in local-tracker repos, login error names fj auth login); live dry runs on taia#253 (SHA256) and harnez#150 behaved; make test-q1 206 ok 0 FAIL (c22f0118, 33cb9b92)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1194 (forge-tracker path), 1064 (issues import), 1067 (issues export), forgetracker `issues new`

---

## 1. Problem & Motivation

Owner request (2026-11-10): add a safe `harnez issues comment` for forge-tracked issues, built on `fj`.
`harnez issues new` already files forge issues through gh/tea/fj (`internal/forgetracker/create.go`),
but commenting needs raw `fj` calls. Seen the same day while commenting by hand:

- `fj` without login fails with `401 Unauthorized`; `fj -H codeberg.org whoami` says "token is required".
- In a SHA256 git repo (taia) `fj -R origin` fails with "remote specified but no git repo found";
  only `-r harnez/taia` and `harnez/taia#253` worked, run outside the repo dir.
- `-H codeberg.org` was needed every time.

## 2. Goal

/goal `harnez issues comment -d <repo> <n> -F <body-file>` (and `--dry-run`) adds one comment to forge
issue `<n>` of the repo's origin through `fj --host <host> issue comment <owner/repo>#<n>`, resolving
host and slug from the origin URL like `issues new` does (works in SHA256 repos), with tests that
check the built command without calling the forge. Stop and ask the owner on a design choice that
changes what "safe" means. On a loop or no progress, clear the goal and write a failure report here.

## 3. Acceptance

- Safe by default: `--dry-run` prints the exact fj command; the real call names the target
  (`host owner/repo#n` and issue title) in its output; body comes from a file (`-F`), never shell-interpolated.
- Refuses pull requests and closed issues unless a flag allows it (unverified whether fj tells them apart;
  check `fj issue view`).
- Missing login: a clear error naming `fj -H <host> auth login`, not a raw 401.
- Works in a SHA256 repo (host/slug from `git remote get-url`, not fj's own repo detection).
- Only fj in this ticket; gh/tea fallback is optional (record if skipped).
- Issue skill's Forge tracker section mentions the command; agents comment on others' tickets only with the owner's OK.

## 4. Notes / Unverified

- Whether `fj issue comment` accepts `--body-file` (its help mentions it) is unverified on this fj version.
- Check the live forgetracker code and recent commits before starting.

## Searcher findings (s1207, codex:luna:low)

- Host/slug: `forgetracker.Resolve` (`internal/forgetracker/forgetracker.go:45`) uses `git remote get-url`
  and `spec/pr.yaml` forges, independent of fj's repo detection. CLI pick: `PlanCreate`, `create.go:35`.
- `issues new` wiring `cmd/harnez/issues.go:359`, `--dry-run` :483, forge path `issues_forge.go:125`.
- fj: `issue comment` takes positional issue, optional body, `--body-file`, `-H`; `fj -H codeberg.org issue
  view harnez/taia#253` works (title, Open state). fj help does not promise PR detection.
- Preflight reuse: `internal/forge/issue.go:12,25` `RemoteIssue` (title, state, pull_request) via
  `PullLister.Issue`; `issues import` (`issues_import.go:33`) and `issues show` (`forgetracker.go:83`) reject PRs.
- Tests: `internal/forgetracker/create_test.go:28` (PlanCreate), help golden `cmd/harnez/help_golden_test.go:13`.

## Decision

- 2026-11-10 host (decide mode): preflight with the existing forge API (`PullLister.Issue`): refuse PRs
  always, refuse closed issues unless `--closed`; print `commenting on <host> <owner/repo>#<n>: <title>`.
  Then run `fj --host <host> issue comment <owner/repo>#<n> --body-file <file>` (exact form to verify
  from fj help; harnez's own `-F` maps to fj's `--body-file`). fj only; gh/tea skipped and recorded.
  Missing fj login (401 / "token is required") maps to an error naming `fj -H <host> auth login`.
  Tests use a fake API and a fake fj on PATH, never the real forge.

## M1 delivered: issues comment with API preflight and fj (c22f0118)

Host: owed full `make test-q1` after the developer's wording fix: 206 ok, 0 FAIL. Live dry runs in taia
(SHA256 repo): closed #253 refused without `--closed`; with it, argv
`fj --host codeberg.org issue comment harnez/taia#253 --body-file <file>`.

## M2 Pre-Work (reviewer r1207, codex:terra:low)

1. P2, host decision: repos with a local `issues/` dir (like harnez itself) may comment on their forge
   issues too, but only with `--forge`, because a bare number there means a local ticket. Without
   `--forge` keep today's refusal and name the flag in the error.
2. P3: dry-run test asserts the exact full argv line; PR and closed-issue cases assert the fake fj was never
   run; add fj "401 Unauthorized" output to the login-error mapping test.
