# 829 — harnez usage is read-only: only the data collector writes the usage store

**Status**: Closed — M2 delivered: local usage views read collector snapshots through a SQLite read-only connection; usage auto-starts the detached server and waits for ready status. M1/M2 tests and installed CLI checks completed; one quota run found only a stale help golden, which was updated and verified with TestHelpGolden.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: 828, 830, 560, 653

---

/goal `harnez usage` (compact, `--watch`, every view) opens the usage store read-only and never writes it; schema setup, migrations, compat imports, observation and history writes happen only in the data collector. A test proves `usage` works against a read-only database file. Stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
Issue 828 found that every `harnez usage` run wrote to the store (schema DDL, key migration, compat
import, observations, snapshots), and those writes caused `database is locked` failures. 828 only
made the writes cheaper and best-effort (ab180545). A reader that writes still contends with writers
and hides who owns the data. One writer, the data collector, keeps locking simple.

## 2. Technical Specification / Findings
- Write paths to move out are in `storeCompactSummary` (`internal/usage/store.go`): `EnsureUsageSchema`,
  `MigrateStableWindowKeys`, `importCompatibility`, `importAgent`, and the history snapshot.
- Open the store read-only (e.g. `mode=ro` / `query_only`) in the `usage` path, so any leftover write
  fails loudly in tests.
- Decided (user, 2026-10-20): when no collector is running, `harnez usage` starts one. Only one
  collector per host is allowed; it holds a fixed local port, so a second start detects the running
  one and exits. `usage` never collects or writes itself. Put the port in `spec/`.
- Decided (user, 2026-10-20): the collector becomes the first service of a new `harnez server`
  process (client-server architecture; `usage`, hooks and statusline are clients). 829 builds the
  server skeleton with the collector service; 830 adds the update endpoint; 831 installs the server.
- Follows from that: the collector runs schema setup and migrations at startup, so a fresh install
  is handled by the collector that `usage` starts.

### Searcher findings (sprint 2026-10-20)
- Today `harnez agent-collector` (`cmd/harnez/usage_cmd.go:402`, `internal/usage/collector.go:26-36`) runs
  `CollectAllLive` every 15m (`spec/usage.yaml:17`) and writes JSON snapshots only; systemd unit from 082
  (`internal/claude/apply.go:89,116`). The SQLite store is written by `usage` compact and `--watch`
  (`internal/usage/store.go:489-586`, `internal/usage/watch.go:658-661`).
- `usage` fetches via `CollectAll` (`internal/usage/usage.go:77-82`): JSON cache, else live provider fetch.
- No general server exists; MCP channel HTTP server (`internal/channel/server.go:68`) and AGY meter
  listener (`internal/agymeter/meter.go:183`) are prior art for listeners. A fixed-port bind is the singleton.

## 3. Milestones
- **M1 — server skeleton with collector service:** new `harnez server` command; fixed localhost port from
  a new spec file (with schema); a bind failure means another server runs, so exit cleanly. The collector
  service does live collection on the spec cadence and owns all SQLite writes (schema, migrations, compat
  import, observations, history snapshots, JSON snapshots). A minimal status endpoint (health + version)
  so clients can detect it. `agent-collector` becomes an alias that runs the server (keep callers working;
  update the 082 unit template to run `harnez server`).
- **M1 delivered (server skeleton) in 91c91ab4.** Host check: `GET /status` answers
  `{"health":"ok","server":"harnez",...}` on 127.0.0.1:18729; a second `harnez server` prints
  "already running" and exits 0; `make test-q1` green.
- **M2 Pre-Work / Required Refinements:** the server's startup log still says `harnez agent-collector:`;
  name the server and its service instead. `duration()` panics on a bad spec value in both
  `cmd/harnez/server_cmd.go` and `internal/server`; validate durations once in `server.LoadConfig`.
- **M2 — read-only usage client with auto-start:** compact and `--watch` read the store read-only and
  never fetch live; if no server answers, start `harnez server` detached, then read. Test against a
  read-only (chmod) DB. Remove the 828 best-effort write paths that no longer apply.

## 4. Implementation & Verification Plan
- Plan first: list every write reachable from `harnez usage` and where each one moves.
- Test: `usage` against a read-only (chmod) store renders rows, and no write is attempted.
- Live check: `harnez usage` and `--watch` while the collector writes; no busy note, no lock errors.
