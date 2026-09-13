# 096 — `harnez release`'s `has_releases` 401 warning doesn't name the fix

**Status**: Closed
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

---

## Implementation Plan

### Findings from the current code

- `GetForgeToken(host string) string` (`internal/release/forge.go:148`) checks, in order:
  `CODEBERG_TOKEN` (for codeberg.org), then `FORGEJO_TOKEN`/`CODEBERG_TOKEN`/`FJ_TOKEN`/
  `GITEA_TOKEN`, then `~/.local/share/forgejo-cli/keys.json`. It **discards which source
  won** — that's precisely the information the hint needs (env var name vs. `fj auth login`).
- `EnsureHasReleases` (`forge.go:192`) returns a flat
  `fmt.Errorf("forge API %s returned status %d: %s", ...)` — the status code is formatted
  into a string and lost. The caller (`runner.go:221`) prints `%v` verbatim.
- Non-2xx handling already special-cases 404 (returns `false, nil`, silent). 401 falls into
  the generic branch.

So the fix needs two small structural changes, not string-sniffing at the print site.

### Steps

1. **`internal/release/forge.go` — expose the token's provenance.**
   Add a source-returning variant and make the existing function a thin wrapper so no
   caller breaks:
   ```go
   // TokenSource describes where GetForgeTokenSource found a token.
   type TokenSource struct {
       EnvVar string // e.g. "CODEBERG_TOKEN"; empty if not from env
       File   string // e.g. "~/.local/share/forgejo-cli/keys.json"; empty if from env
   }

   func GetForgeTokenSource(host string) (string, TokenSource)
   func GetForgeToken(host string) string { t, _ := GetForgeTokenSource(host); return t }
   ```
   Move the existing body into `GetForgeTokenSource`, recording the winning branch.

2. **`internal/release/forge.go` — make the API error typed.**
   ```go
   type ForgeAPIError struct {
       URL        string
       StatusCode int
       Body       string
       APIMessage string // parsed from the Gitea/Forgejo JSON {"message": "..."} body
   }
   func (e *ForgeAPIError) Error() string  // same text as today, so output is unchanged
   func (e *ForgeAPIError) IsAuth() bool   // 401 || 403
   func (e *ForgeAPIError) IsExpired() bool // IsAuth() && APIMessage contains "token is expired" / "invalid claims"
   ```
   Return `&ForgeAPIError{...}` from the non-2xx branch (`forge.go:219`) instead of
   `fmt.Errorf`. Best-effort JSON decode of the body into `struct{ Message string }`;
   on decode failure leave `APIMessage` empty and let `IsAuth()` alone drive the hint.

3. **`internal/release/runner.go` (~line 217-222) — emit the hint.**
   Capture the source alongside the token, and after printing the existing warning, add a
   second line only when `errors.As(err, &apiErr) && apiErr.IsAuth()`:
   ```
   [forge]     Warning: failed to check 'has_releases' unit: <err>
   [forge]     Note: this check is non-fatal; the release will continue.
   [forge]           Refresh the token with `fj auth login`.
   ```
   …or, when `src.EnvVar != ""`:
   ```
   [forge]           The token came from $CODEBERG_TOKEN — update or unset that
   [forge]           env var (unset it to fall back to `fj auth login`'s keyring).
   ```
   Non-auth errors (network, DNS, 5xx) print exactly what they print today — **no hint**,
   which is the whole point of the typed error.

4. **Tests — `internal/release/forge_test.go`.**
   - `httptest.Server` returning 401 with a Gitea-shaped body: assert `EnsureHasReleases`
     returns a `*ForgeAPIError` with `StatusCode == 401`, `IsAuth() == true`,
     `IsExpired() == true`, and that `Error()` still contains the URL and status (so the
     existing warning text is unchanged).
   - 500 with a plain-text body: `IsAuth() == false`, `APIMessage == ""` (no panic on
     non-JSON body).
   - 404 still returns `(false, nil)`.
   - `GetForgeTokenSource` with `t.Setenv("CODEBERG_TOKEN", "x")` → `EnvVar ==
     "CODEBERG_TOKEN"`; with env cleared and a temp `HOME` holding a `keys.json` →
     `File` non-empty, `EnvVar == ""`.
   - A runner-level test asserting the hint text appears for 401 and does **not** appear
     for 500, writing to an `opt.Out` buffer. If `runRelease`'s preflight isn't
     independently callable, extract the ~8-line forge block into
     `func checkForgeReleases(out io.Writer, forge *ForgeInfo, dryRun bool)` and test that.

### Design decisions / tradeoffs

- **Typed error over string matching.** Matching `strings.Contains(err.Error(), "401")` at
  the print site would work today and break the first time a body happens to contain "401".
  The typed error is ~30 lines and makes the 403 case (valid token, missing scope — where
  `fj auth login` is *also* roughly the right advice) explicit rather than accidental.
- **Treat 403 as auth too**, but word the hint as "refresh or re-scope the token" rather
  than asserting expiry, unless `IsExpired()`.
- **Don't suppress the raw error.** The user should still see the API body; the hint is
  additive. Keeping `Error()`'s text identical also means no golden-output test churn.
- **Don't try to auto-run `fj auth login`.** Interactive, and the release is mid-flight.

### Risks / open questions

- Forgejo's error `message` wording for an expired token is not a stable API. Mitigation:
  `IsExpired()` is only used to choose between "refresh it" and "re-scope it" phrasing —
  the *hint at all* is gated on the HTTP status, which is stable.
- If both an env var and `keys.json` hold tokens, the env var wins silently today; the new
  source reporting makes that visible, which may itself surprise a user. That's an
  improvement, not a regression.

### Scope

**Small** — ~60 lines of production code across two files plus a test file. P3, good
candidate for a drive-by fix next time `internal/release` is touched.
