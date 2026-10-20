# 830 — Hooks and statusline scripts hand usage updates to the data collector instead of writing the store

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: 829, 828, 653, 560

---

/goal Hooks, statusline scripts and other passive observers never open a write transaction on the usage store. They hand each update to the data collector, which stores it or drops it (duplicate, stale, collector busy), and the observer never blocks on a DB lock. Stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
Statusline observations write to the store directly (`internal/usage/passive.go`, 25ms busy timeout),
so they compete with the collector and with `harnez usage` for the write lock (see 828). Observers run
on every prompt or render and must stay fast. With one writer, the collector, locks are never contended.

## 2. Technical Specification / Findings
- The current direct writers include the statusline passive observation path (`internal/usage/passive.go`,
  `WriteCurrent` in `internal/usagestore/store.go`). Find any other hook-side writers in the plan.
- The collector already decides what to keep (passive dedupe interval in `spec/usage.yaml`).
- Decided (user, 2026-10-20): observers send updates to the running collector service, on its fixed
  per-host port from 829. When no collector answers, the observer drops the update (decided by the
  user, 2026-10-20); it never starts a collector and never blocks.

### Pre-Work from 829 (host review 2026-10-20)
- After 829 M3 (b9e63900) the `harnez usage` header shows `19:42:08 UTC`; before it showed local time
  (`21:19:52 CEST`). Show local time again; add a test that pins the timezone.

### Searcher findings (sprint 2026-10-20)
- Usage writer: `harnez statusline` (`cmd/harnez/statusline.go:41`) → `ObserveStatusline`
  (`internal/usage/passive.go:46`) → `WritePassive`/`WriteCurrent` (`internal/usagestore/store.go:488-528`),
  on every status render. Dedupe lives in `WritePassive` (`passive_dedupe_interval: 30s`).
- Same SQLite file, other telemetry writers: `harnez rate` (`cmd/harnez/rate.go:148`), `harnez hook agy` /
  `hook post-tool` (`cmd/harnez/hook.go:140`), `harnez codex-telemetry` (`cmd/harnez/codexhooks.go:101,165,240`).
  All use `telemetry.DefaultDBPath()` = `$XDG_DATA_HOME/harnez/telemetry.sqlite`, the usage store file too,
  so they contend for the same lock. The user's ask covers "hooks, statusline scripts, etc.", so they are in scope.
- `internal/server` has only `GET /status` on a private mux; no route registration for services yet.
- `statusline_busy_timeout` / `statusline_write_timeout` become obsolete once statusline stops writing.

## 3. Milestones
- **M1 — update endpoint:** Pre-Work above (local time) first. Services can register routes;
  the collector serves `POST` for passive usage windows and applies the existing dedupe. A client helper
  sends with a short spec timeout and drops the update when no server answers (never starts one).
- **M1 delivered (update endpoint) in 0cee90eb.** Host check: header shows `21:50:06 CEST`; killing the
  server and running `harnez usage` auto-started a new one; malformed and empty POSTs return 400.
- **M2 Pre-Work:** `PassiveUpdateServer.RegisterRoutes` silently skips the route when `usageDurations()`
  fails; load the spec once at server start and fail the start with the error instead.
- **M2 — statusline on the endpoint:** statusline uses the helper; remove its direct DB open/write and
  the obsolete spec keys (yaml, schema, loader, tests). Document the client-server architecture in
  `docs/UsageCollection.md` (and `docs/HarnezComponents.md`): `harnez server` is the single writer;
  `usage` reads read-only and auto-starts the server; observers POST and drop on failure (usage) or
  spool (telemetry, M3); one server per host on the spec port; systemd service per 831.
- **M2 committed in 8fbc23f8, not accepted (live write fails).** Host check 22:55, server restarted on
  the installed 8fbc23f8 binary, `/status` ok: every valid `POST /usage/passive` (e.g. `{"Windows":[{
  "Provider":"claude","Key":"five_hour","Source":"probe","ObservedAt":"<now>","UsedFraction":0.42}]}`)
  returns `503 collector write failed` within ~1 ms. It is not lock contention, which would wait out
  the busy timeout. The M1 server failed the same way, so M1's tests also missed it.
  **M2 rework:** (a) the server logs the write error (stderr, and a log file under the state dir when
  detached) instead of only answering 503; (b) find and fix the cause; (c) add a test that posts
  through the real HTTP route into a real store opened the way `harnez server` opens it in production
  (real path setup, schema and migrations), then reads the row back; (d) live: restart the server
  from the installed binary, POST, and query the row read-only. Also decide whether the wire format should
  have lower-case JSON tags; if so, change it in the handler, the client and the tests together.
- **M2 delivered (statusline on the endpoint) in 8fbc23f8 + rework 62df1c68.** Cause: the store writer got
  an empty DB path while init resolved the default. Host check 22:56: lower-case POST returns 204 and the row
  is in `usage_observations` (read-only query); write failures now go to stderr and `collector.log`.
- **M3 Pre-Work:** 62df1c68 added snake_case JSON tags to `usagestore.Window`; 2 of 1855 lines in
  `~/.harnez/agents/quota-readings.jsonl` (read by `legacyarchive.go`) use the old CamelCase keys, and Go's
  case-insensitive match does not map `UsedFraction` to `used_fraction`. Decode both forms (or prove the
  reader ignores those fields) with a test.
- **M3 — telemetry hooks on the server:** `rate`, `hook post-tool`/`hook agy` and `codex-telemetry` send
  their records to a server telemetry route instead of writing the DB. Decided (host, delegated by the
  user, 2026-10-20): when no server answers, telemetry records are **not dropped** but appended to a
  bounded local spool (one small file per record under the harnez state dir; size cap and path in spec),
  which the server drains into the DB at startup and on each tick. Reason: these records are history, so
  a drop leaves a permanent gap, while a dropped usage reading is replaced by the next one. Passive usage
  windows (M1/M2) keep the drop rule. Spool writes are plain file creates, never a DB lock. Afterwards only `harnez server` writes `telemetry.sqlite` (add a test or check that greps
  for write calls outside the server packages).

## 4. Implementation & Verification Plan
- Plan first: list every observer that writes, and choose the hand-off with its no-collector behaviour.
- Test: an observer completes within its time budget while a blocker holds the DB write lock, and the
  collector later stores (or deliberately drops) that update.
- Docs: record the single-writer rule in the usage architecture doc.
