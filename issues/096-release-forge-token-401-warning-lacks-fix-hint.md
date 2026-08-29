# 096 — `harnez release`'s `has_releases` 401 warning doesn't name the fix

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: `internal/release/forge.go`, `internal/release/runner.go`, [[091-language-agnostic-release-spec-and-thin-make-release]], `docs/practices/GoRelease.md`

## Problem

When the token `GetForgeToken` resolves (env vars, falling back to `fj`'s own
`~/.local/share/forgejo-cli/keys.json`) is expired, the `has_releases`
preflight check in `harnez release` fails and prints:

```
[forge]     Warning: failed to check 'has_releases' unit: forge API
https://codeberg.org/api/v1/repos/<owner>/<repo> returned status 401:
{"message":"token has invalid claims: token is expired\n...
```

This is non-fatal — the release itself continues and normally succeeds,
since the actual `git push` and `fj release` publish step authenticate
separately. But the warning gives no hint that (a) it's safe to ignore for
this run, or (b) the fix is `fj auth login`. Confirmed 2026-08-29: a user
had to be told this was non-fatal and had to run `fj auth login` by trial
rather than being pointed there by the tool.

## Proposed Fix

In `internal/release/runner.go` where this warning is printed (around line
187), detect a 401/expired-token error specifically (vs. other API
failures) and append a one-line hint, e.g.:

```
[forge]     Warning: failed to check 'has_releases' unit: token expired.
            Release will continue; run `fj auth login` to refresh it for
            next time.
```

Requires distinguishing "token expired/invalid" (safe to hint at
`fj auth login`) from other failure modes (network error, repo doesn't
exist, permissions issue) where that hint would be wrong — parse the
Forgejo/Gitea error body's `message` field or HTTP status rather than
just checking for non-2xx.

## Notes

Root-caused this session via `docs/practices/GoRelease.md`'s §3 update
(same investigation): the check's token isn't a separate credential to
manage, it's whatever `fj auth login`/`fj auth add-token` last wrote,
unless an env var overrides it. The fix suggestion above should stay
consistent with whichever source actually produced the expired token
(if an env var is set, `fj auth login` won't help — the hint should name
the env var instead in that case).
