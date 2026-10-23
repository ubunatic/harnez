# Two-agent lean sprint: usage never hangs (issue 873)

Date: 2026-10-23. Host: Claude Opus. Searcher: `search-873` (luna:low). Developer: `dev-873` (luna:high).

## Flow

- Searcher (47s): every blocking call in `harnez usage` with file:line and timeouts; `--minimal` waited up to
  30s on `ensureLocalServer`. Findings and milestones went into the ticket.
- Plan turn (1m17s), then host added: no second server next to a slow listener, reproduction test first,
  status on stderr.
- M1 (`1834d3af`, 22m): one 1s probe, start only when nobody listens, 10s `usage_deadline` in spec, binary
  tests for no server, starting, hung, healthy (2-3s each). Found a second `/status` call with the old 5s
  timeout during its own test runs. Time went mostly into six `make test-q1` runs, one blocked by another
  session's uncommitted ticket files.
- M2 (`d04d7673`, 11m): `-w` status row, `/status` reason, store read bounded by the deadline (FIFO store
  test: error after 270ms with a 250ms test deadline), pty test against a hung socket.

## Verification by host

Installed binary on the real store: `--minimal` 5.7-8.2s, also right after a server restart; no hang.
`--offline` 8.2s once: the store read (874) uses most of the 10s budget, a busy host may hit the deadline.
Developer's split (fixture only): process start 0.01s, offline JSON 0.02s, `--minimal --offline` 2.0s;
the live 6-8s did not reproduce on the fixture, so 874 needs a live-store profile.

## Learnings

- Bounded probe tests found the hidden second request; a timing test per server state pays off.
- A shared tree with other sessions' uncommitted files blocks the quota-1 test rule; workers waited ~2m.
