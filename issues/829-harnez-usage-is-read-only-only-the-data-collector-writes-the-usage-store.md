# 829 — harnez usage is read-only: only the data collector writes the usage store

**Status**: Closed — M3 delivered in b9e63900; test-q1 green; installed usage showed 19:42:08 UTC · 0m ago and Claude weekly tokens 18,370,886
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
- **M2 delivered (read-only client) in 4de0d881**, `make test-q1` green on host rerun. Host live check found:
- **M3 Pre-Work / Required Refinements (reopened 2026-10-20):**
  1. Stale header: server (started 21:31:35) wrote JSON snapshots at 21:31:41, but `harnez usage`
     showed `21:19:52 · 15m ago · stale`. Find which source the client reads and why the collector's
     fresh data does not reach it (DB not written at startup? reads a different file/DB?).
  2. Implausible tokens: Claude weekly tokens `4,230,853,717` vs `18,370,886` before M2. Find the cause
     (double counting, summing all history rows, wrong field) and fix it; add a test with known totals.
  3. Dead write/collect paths: `StoreCompactSummary*` (`internal/usage/store.go:23-32`) is now only used
     by tests, and `watch.go:657`, `watch_frame.go:535`, `loom.go:88` still fall back to live `CollectAll`
     when no loader is set. Move what the collector needs into the collector and delete the rest, so a
     client cannot write or live-fetch by accident.
- **M3 — verify:** host reruns `harnez usage` (×3), `--json`, and `-w` in a pty; header time matches the
  newest collector write; totals match the pre-829 magnitude.

## 4. Implementation & Verification Plan
- Plan first: list every write reachable from `harnez usage` and where each one moves.
- Test: `usage` against a read-only (chmod) store renders rows, and no write is attempted.
- Live check: `harnez usage` and `--watch` while the collector writes; no busy note, no lock errors.
