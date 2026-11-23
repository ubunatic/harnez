# 1276 — `harnez pr create` should fall back to AGit flow when token access fails

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 1273, 1247, 1062

---

/goal Make `harnez pr create` fall back to the AGit SSH push flow (`git push <remote> HEAD:refs/for/<base>/<topic>` with title/description push options) on AGit-capable forges (Forgejo/Codeberg/Gitea) when API token access is missing or fails (e.g. HTTP 401/403 lacking `write:repository`), with tests and `--dry-run` support, or stop and report when blocked on a user decision or denied permission. On a loop, hard blocker, or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

In `harnez pr create` (added in #1273), creating a pull request via the Forgejo REST API requires a token with `write:repository` scope. When a host has SSH push access and a read-only or `write:issue`-only `CODEBERG_TOKEN`, `harnez pr create` fails with:

```
Error: forge API request failed: HTTP 403 token does not have at least one of required scope(s): [write:repository]
```

Meanwhile, Forgejo/Codeberg natively supports creating PRs over SSH via the AGit flow (`git push origin HEAD:refs/for/<base>/<topic> -o topic=<topic> -o title=<title> -o description=<body>`), which requires no API write token at all.

## 2. Desired Behavior

- When `harnez pr create` targets a Forgejo/Gitea/Codeberg remote and the API call fails due to missing token or `401`/`403` scope error (or when `--agit` is explicitly requested), fall back to pushing via AGit (`refs/for/<base>/<topic>`).
- Pass `--title` and `--body` via AGit push options (`-o title=...`, `-o description=...`), deriving `<topic>` from `--head`/current branch (or an explicit `--topic` flag).
- Respect `--dry-run` (print the AGit push command/refspec without pushing) and host-write guardrails.
- Print the resulting PR URL/number parsed from the remote's push output.
- On non-AGit forges (e.g. GitHub), keep returning the actionable token/scope error.

## 3. Acceptance Criteria

- `harnez pr create` on a Forgejo/Codeberg repo falls back to AGit when no write token is available or the API returns 401/403.
- Unit tests cover API-success path, 401/403 -> AGit fallback, `--dry-run`, and non-AGit forge error path.
- `docs/AGit.md` / CLI help updated if new flags (e.g. `--agit` / `--topic`) are added.
