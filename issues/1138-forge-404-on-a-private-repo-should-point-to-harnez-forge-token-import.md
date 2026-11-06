# 1138 — Forge 404 on a private repo should point to harnez forge token import

**Status**: Closed — Review pre-work: added TestGetJSONAuthFailuresWithTokenAreNotRetried (401 and 500 with a token make one request, no anonymous retry) and TestGetJSONForbiddenThenAnonymousNotFoundKeepsScopeAdvice (403 with token then anonymous 404 returns 403 scope advice). Both pass on the current code; go test ./internal/forge passes (commit 8c6e875b).
**Closed-By**: claude:haiku:med, session dev-1138
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1116, 1118, 1139; taia (codeberg.org/harnez/taia)

---

## Problem

Seen 2026-11-06 with no token stored on this machine:

```
$ harnez find -d ../taia issues plugin
Error: find issues: codeberg.org/harnez/taia: forge API request failed: HTTP 404 API request to
codeberg.org (a private repo needs a token: set one of CODEBERG_TOKEN, FORGEJO_TOKEN)
```

The hint names only the env vars. The intended route since 1118 is the stored token
(`harnez forge token create` or `import`), which `forge.TokenSource` reads first.
`harnez forge token status` said "No token stored for codeberg.org" although 1118 recorded a
stored token for taia (another machine or keyring).

## Second symptom: stored token breaks a public repo read

After `harnez forge token import --user ubunatic --check harnez/taia -` (no `--name`, scopes
not listed), `harnez issues new -d .` in harnez printed:

```
open PRs not checked on origin (forge API request failed: HTTP 403 API request to codeberg.org)
```

Before the import, that check on the public `ubunatic/harnez` repo worked without a token. The
stored token is sent to every repo on the host, and a 403 from a token that lacks a scope is not
retried anonymously. Not verified: which scope is missing (Forgejo lists scopes only with the
password).

## Want

- The 404 hint names `harnez forge token import` / `create` first, then the env vars.
- On a 403 with a stored token, either retry without the token for a public repo, or say which
  scope the token lacks and how to rotate it.

## Done when

- Tests cover both messages, and the 403 case on a public repo no longer drops the open-PR check.

## Decision

- 2026-11-06 lean sprint: on HTTP 403 for a read request sent with a stored or env token, retry once
  without the token (public repos need none); if that also fails, report that the token lacks a scope
  and name `harnez forge token rotate` / `import`. Reason: keeps the open-PR check working for public
  repos without asking the owner to widen a read-only token.

## Plan accepted (dev-1138, claude:haiku:med, 2026-11-06)

One GET-only helper in `internal/forge` used by `getJSON` and `prwait.Fetch`: token + 403 -> one
retry without `Authorization`; on a second failure return the original 403 (a private repo answers
404 anonymously, which would trigger the wrong hint). No retry on 404/401/5xx or without a token. Scope
advice in `APIError.Detail` (Forgejo: `harnez forge token rotate`/`import`; GitHub: `GITHUB_TOKEN`/`gh
auth`). 404 hint is kind-specific. 9 tests with a sentinel token. Live check: host runs `issues new`
in a scratch clone after the commit.

## Review (review-3fixes, codex:terra:low) and pre-work

No blocker, no secret leak; the retry cannot loop; a 403 body is discarded before decoding. Missing
tests for promised behaviour (`internal/forge/getjson_test.go`):

- An authenticated 401 and an authenticated 5xx make exactly one request (no anonymous retry).
- An authenticated 403 followed by an anonymous 404 returns status 403 with the scope advice, not the
  404 (the private-repo case).
