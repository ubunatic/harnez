# 991 — harnez usage fails with no collector snapshots and cannot show Codex free-plan quota

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 829, 714, 942, 954, 955

---

/goal `harnez usage` gives a useful answer on a fresh machine and for a Codex account on the
ChatGPT free plan: no help-text dump when the store is empty, and a Codex row that shows the
free-plan quota (or says why it cannot). Verify live on a host with the free plan, or stop and
report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard
architectural or OS blocker, or no progress, clear the goal and write a failure report here
(what was tried, why it failed, resume point).

## Observed (2026-10-28, harnez 0.2.4, found while bootstrapping forgewerk)

- `harnez usage` (also `--compact`, `--agent codex`) prints
  `Error: read usage store: usage store has no collector snapshots yet` and then the full usage
  help. Exit status still 0.
- `harnez server status`: `harnez-agent-collector.service (installed=false, active=false)`, yet a
  `harnez server` process (PID 25109, started 13:37:34) owns the socket. Its uptime was over
  4 minutes and there was still no snapshot.
- Codex is logged in via ChatGPT on the free plan (`codex login status`: "Logged in using
  ChatGPT"). That plan has a small quota, and harnez has no way to show it.
- `harnez agent models --picks`: availability is `unknown` for every model. The default picks
  resolve to codex (luna/terra/sol) even when its quota is tiny or used up, with no fallback.

## Expected

1. Empty store: one short message saying what is missing and how to fix it (collector not
   installed or no first fetch yet; `harnez server install`), no help dump, non-zero exit.
   Consider having a running server do a first collection right at startup.
2. Codex on the free / ChatGPT-login plan: the collector reads the quota or marks the row
   `unsupported plan` with a reason. A failed parse must not drop the store.
3. Picks: once usage knows a vendor is exhausted or logged out, `--picks` and `harnez agent`
   should skip to the next model in the chain. Access discovery itself belongs to 942
   (954/955); this ticket only connects quota state to it.

## Notes

- 829 made `harnez usage` read-only (only the collector writes), so a missing collector now
  makes `harnez usage` fail completely.
- Check the live code and recent history first; 942's children may already cover item 3.

## Findings (2026-10-28, after `harnez server install`)

- **Install leaves the unit unable to start.** The unit was enabled but failed with
  `status=226/NAMESPACE`: `Failed to set up mount namespacing: /home/alfiz/.cache/harnez: No such
  file or directory`. Every `ReadWritePaths=` entry (`%t/harnez %S/harnez %C/harnez
  %h/.local/share/harnez`) must exist, and `%C/harnez` did not. Fix: `harnez server install`
  creates the directories, or the unit prefixes the entries with `-`. Creating the directory by
  hand fixed it.
- **Stray server holds the socket.** A `harnez server` that had not been started by systemd (it
  had 0 clients and never wrote a snapshot) owned the socket, so the unit could not take over
  until that process was killed. Then `systemctl --user restart` worked, and the first snapshot
  arrived within seconds. Check why the stray server never collected anything.
- **Codex root cause.** `harnez usage --agent codex --json` shows `"authenticated": false,
  "error": "invalid auth.json format"`. `CodexAuth.Tokens` is `map[string]string`
  (`internal/usage/codex.go:21`), but the current Codex CLI writes a nested object
  `tokens.oauth_provenance {issuer, client_id}` (alongside `id_token`, `access_token`,
  `refresh_token`, `login_owner_id`, `account_id`), so `json.Unmarshal` fails for every ChatGPT
  login. Fix: give the tokens a struct with string fields and ignore unknown nested keys; add a
  test fixture with `oauth_provenance`. Only after that can you tell whether the free plan's
  rate-limit response is parsed correctly (Expected item 2).

## Progress (2026-10-28)

- Done: `CodexTokens` struct replaces the token map (`internal/usage/codex.go`); regression test
  `TestCollectCodexParsesNestedTokenFields`. Live after `make install`: Codex authenticated,
  `plan_tier: Free`, one window at 0% used, resetting 2026-11-26 (~29 days out).
- New: the free plan reports a single ~30-day window, which `buildCodexQuotaWindow` labels
  `Weekly`. Name windows by their real length (e.g. `Monthly`), and confirm the free plan never
  has a 5-hour window.
- Unrelated, already on main: `TestCollectCatchupFetchesOnlyProvidersOlderThanThreshold` has a
  data race under `-race` (`catchup_test.go:39/51`), so `make test-pkg PKG=./internal/usage/...`
  fails without this change too. File it separately if no ticket covers it.
- Still open: Expected items 1 (empty-store message, unit dirs, stray server) and 3 (picks
  fallback, see 992).
