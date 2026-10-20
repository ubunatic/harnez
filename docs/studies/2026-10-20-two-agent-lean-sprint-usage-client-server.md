# Two-Agent Lean Sprint: usage client-server split (828-830)

**Date**: 2026-10-20 · **Host**: Claude Opus 5.5 · **Workers**: codex luna:low (searchers), luna:high (828), sol:low (829, 830)
**Related**: issues 828, 829, 830, 831, 833, 837; [HarnezServerArchitecture.md](../HarnezServerArchitecture.md), [Databases.md](../Databases.md)

## Outcome

| Ticket | Result | Commits |
|---|---|---|
| 828 locked DB in `harnez usage` | Stopgap: skip finished migrations, best-effort writes | ab180545 |
| 829 usage read-only | `harnez server` + collector service (M1), read-only client with auto-start (M2), fresh header and token totals (M3) | 91c91ab4, 4de0d881, b9e63900 |
| 830 observers POST to the server | M1 endpoint 0cee90eb; M2 statusline and M3 telemetry spool in progress | 0cee90eb |

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

## Efficiency

- 828: ~7 min developer, 170k new tokens. 829 M1-M3: ~25 min developer, ~1.3M new tokens (M3 alone
  658k). 830 M1: ~4 min, 137k. Searchers: under 1 min each.
- The single largest cost was 829 M3, a rework round that a live check at the end of M2's own
  turn (not just pasted output) would have shortened.
