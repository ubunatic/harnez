# Two-Agent Lean Sprint: usage client-server split (828-830)

**Date**: 2026-10-20 · **Host**: Claude Opus 5.5 · **Workers**: codex luna:low (searchers), luna:high (828), sol:low (829, 830)
**Related**: issues 828, 829, 830, 831, 833, 837; [HarnezServerArchitecture.md](../HarnezServerArchitecture.md), [Databases.md](../Databases.md)

## Outcome

| Ticket | Result | Commits |
|---|---|---|
| 828 locked DB in `harnez usage` | Stopgap: skip finished migrations, best-effort writes | ab180545 |
| 829 usage read-only | `harnez server` + collector service (M1), read-only client with auto-start (M2), fresh header and token totals (M3) | 91c91ab4, 4de0d881, b9e63900 |
| 830 observers POST to the server | Endpoint (M1), statusline + empty-path rework (M2), telemetry route + spool (M3), single writer with boundary test (M4) | 0cee90eb, 8fbc23f8, 62df1c68, c7848059, c6eb523d |
| 837 Unix socket | HTTP over `$XDG_RUNTIME_DIR/harnez/server.sock`, newest build takes over | 31f04748 |
| 833 quit keys | Escape sequences parsed as one key; only q, Q, Ctrl-C quit | 7a74997c |
| 831 systemd user service | `harnez server install/status/uninstall`; not enabled on this host (user's choice) | d63ffbc2, b53bb92b, ef81314e, 2000ee11 |

Released as v0.1.34 (2026-10-21).

## What worked

- **Searcher first** (luna:low, ~40 s, ~50k tokens each) gave file:line maps that made every
  developer plan concrete on the first turn.
- **Plan-only first turn**: the host rejected two weak plans before any code (828: "make the 5 s
  timeout configurable", which changed nothing; 830 M1: passive writes queued behind 2-minute fetches).
- **Host live checks** found three bugs the green suite missed (829 M3, below).

## What failed

- **Green tests, wrong output** (829 M2): stale header time (the reader used a throttled history row)
  and Claude tokens 4.2 B instead of 18 M (cache tokens counted). Caught only by running the
  installed binary. Now in `ReviewLearnings.md`.
- **Developer untested edits**: 828 and 829 M2 workers edited after their single quota run; the
  host's own `make test-q1` rerun was needed both times.
- **Missed prior design**: `docs/HarnezServerArchitecture.md` (a `harnez serve` proposal from the same
  day) was not read before building `harnez server`; the divergence is issue 837.
- **Lock caveat**: 828's schema marker (`usage-schema-v1`) skips future `EnsureSchema` additions
  unless the marker is renamed; superseded once only the server writes.

- **Hung dispatch** (830 M2): a background job chaining `git commit; harnez agent resume` sat 50 min on
  a `harnez statusline` child waiting for stdin; M2 never started. Fix: one step per job, `</dev/null`
  on every harnez call, and check job progress at the first check-in instead of trusting "running".
- **Live checks keep finding what tests miss**: 830's passive route had failed every write since M1
  (empty DB path, error swallowed); 831's `status` saw no socket owner while a server ran. Both had
  green suites.
- **Workers committing on red**: 831 committed with a failing fake-`systemctl` test, and 837 edited tests
  after its only quota run. Host reruns caught both.
- **Dead worker** (830 M3): the d830 worker died mid-turn after ~47M session tokens with 18 uncommitted
  files; a fresh developer (d830b) finished it from the diff and ticket in ~6 min. Long-lived developer
  sessions are worth replacing between milestones.
- **Parallel developers in one tree** (830 M3 and 833) worked when each staged only its own paths, but
  the quota rule blocked 833's gate while 830's files were uncommitted, and each run failed on the
  other's tests. Serialize the test gates or commit one side first.
- **Host error**: `harnez agent wait` takes the session as an argument, not `--name`; `agent delete` has
  no `--name` (use `--all-completed`). Both cost a round.

## Efficiency

- 828: ~7 min developer, 170k new tokens. 829 M1-M3: ~25 min developer, ~1.3M new tokens (M3 alone
  658k). 830 M1: ~4 min, 137k. Searchers: under 1 min each.
- The single largest cost was 829 M3, a rework round that a live check at the end of M2's own
  turn (not just pasted output) would have shortened.
