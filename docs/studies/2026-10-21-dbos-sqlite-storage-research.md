# DBOS Go + SQLite as Harnez storage — issue 841 M1

Research date: 2026-10-21. DBOS was pinned to v1.7.0. The canary source and its binaries/databases are outside this repository at `/tmp/claude-1000/-home-uwe-projects-harnez/471e7cd0-6fbf-43b9-a74d-0b45ca76d587/scratchpad/dbos-canary/`.

## Recommendations

- **(a) Durable `harnez serve` agents: conditional GO for a contained prototype, not yet a production adoption.** DBOS recovered a workflow from its last completed step after a forced process kill, and its workflow, step, queue, timer, schedule, cancellation, event and idempotency APIs overlap with the planned ledger. Keep DBOS inside one long-lived server owner, with clients talking over the existing service boundary. Before committing to it, resolve the measured SQLite startup contention and prototype Harnez's provider-turn boundary, leases/fencing and uncertain external effects. If the design requires multiple Harnez processes to initialize the same DBOS SQLite database concurrently, this canary is a NO-GO for that deployment shape.
- **(b) General Harnez state: NO-GO.** Keep the current plain SQLite and file stores. Most are short-lived CLI state or ordinary queryable data and do not need durable workflow replay. DBOS adds about 12.6 MB to the measured binary, about 26 ms of warmed context lifecycle time, DBOS-owned migrations and a multi-process SQLite startup failure in the one N=8 trial.

This recommendation is limited to this pinned release, one Linux machine, one canary run, and the exact code and build method below. It does not establish production throughput, power-loss durability, filesystem portability, or multi-host behavior.

## Upstream facts verified

- The official Go SDK supports SQLite through the optional blank import `github.com/dbos-inc/dbos-transact-golang/dbos/driver/sqlite`; it registers `modernc.org/sqlite` and needs no cgo. The SDK can also be used with Postgres/CockroachDB. Harnez already uses `modernc.org/sqlite` v1.57.0.
- DBOS Go v1.7.0 was released 2026-10-15. The checked module declares Go 1.25, uses the MIT license, and directly depends on pgx, cron, Cobra, Viper and `modernc.org/sqlite` (v1.54.0 in its module file). The canary raised SQLite to v1.57.0 to match Harnez; normal Go minimum-version selection handles that. Releases 1.0 through 1.7 show an active v1 release line, but do not by themselves establish a support policy or long-term compatibility guarantee.
- `NewContext` connects to the system database, creates or checks its schema, runs pending migrations and pings it. SQLite can be selected with a URL or a supplied `*sql.DB`. The default SQLite pool maximum is eight connections. This means each process that starts a DBOS context may participate in schema checks/migrations; callers should not assume a warmed DB makes concurrent startup contention disappear.
- A workflow consists of deterministic code and checkpointed steps. Reopening the application recovers pending workflows from the last completed step. A caller-supplied workflow ID acts as an idempotency key for workflow admission. DBOS also has queues, durable sleep, schedules, workflow events/messages, streams and workflow cancellation. `RunAsTransaction` can commit application writes and a DBOS checkpoint atomically when the same database handle is used.

Sources: [DBOS Go releases](https://github.com/dbos-inc/dbos-transact-golang/releases), [v1.7.0 module file](https://github.com/dbos-inc/dbos-transact-golang/blob/v1.7.0/go.mod), [license](https://github.com/dbos-inc/dbos-transact-golang/blob/v1.7.0/LICENSE), [SQLite configuration and startup](https://docs.dbos.dev/golang/reference/configuration), [workflows and steps](https://docs.dbos.dev/golang/reference/workflows-steps), [workflow recovery and IDs](https://docs.dbos.dev/golang/tutorials/workflow-tutorial), [queues](https://docs.dbos.dev/golang/reference/queues), [durable methods and sleep](https://docs.dbos.dev/golang/reference/methods), [workflow communication](https://docs.dbos.dev/golang/tutorials/workflow-communication), and [transactions/data sources](https://docs.dbos.dev/golang/reference/datasources).

## Canary method and results

Machine: Fedora Linux 7.2.8, x86_64, AMD Ryzen 5 PRO 5650U (6 cores / 12 threads). Go: `go1.26.8-X:nodwarf5`, linux/amd64. Build used default `go build` settings without `-trimpath` or stripping. DBOS was v1.7.0; both binaries used modernc SQLite v1.57.0.

### Crash and resume

The workflow had three ordered steps. Step 1 wrote a marker; step 2 wrote a start marker, slept for 20 seconds and wrote a completion marker; step 3 wrote its marker. I started the workflow, waited for `step2-started`, sent SIGKILL to that exact process, restarted the same app against the same database and retrieved the same workflow ID. DBOS logged `Recovered pending workflows count=1`; the resumed run completed. The event file contained `step1` once, `step2-started` twice, `step2-completed` once and `step3` once. This demonstrates completed-step replay avoidance and re-execution of the interrupted step for this local process-kill case. The step's file append is a canary observation, not proof that arbitrary external side effects are exactly once.

The first attempt used Ctrl-C and left the original process running long enough to complete step 2; that run was discarded. The result above uses SIGKILL and a fresh database.

### Startup lifecycle time

The canary timer starts immediately before opening the SQLite handle and stops after `NewContext`, `Launch`, `Shutdown` (10-second timeout) and closing the handle. It excludes process startup and Go package initialization. I ran 12 cold samples, each against a new database requiring initial schema setup, and 12 warm samples in separate processes against one already-initialized database. One preliminary warm-up was excluded. Values are milliseconds:

| Run | n | Median | Range |
|---|---:|---:|---:|
| Cold, new DB | 12 | 77.1 ms | 69.300–96.207 ms |
| Warm, existing DB | 12 | 26.3 ms | 24.993–26.436 ms |

These are context lifecycle timings, not full CLI command latency. The 26.3 ms warm median is still a per-process cost if a short-lived CLI command opens DBOS on each invocation.

### Binary size

Built `dbos-canary` (DBOS context plus workflow) and a separate SQLite-only hello program from the same module, Go version and default build settings. `stat` reported:

| Binary | Bytes |
|---|---:|
| DBOS + SQLite | 22,155,384 |
| SQLite-only | 9,578,006 |
| Difference | 12,577,378 (~12.0 MiB) |

This is a simple pair of binaries, not a Harnez before/after build. Exact size will change with link flags, target and other imports.

### Eight concurrent processes opening one database

I initialized a database first, then used `xargs -P8` to start eight independent canary processes concurrently. Each opened the same SQLite file, initialized `NewContext`, launched and shut down. The canary supplied a shared `*sql.DB` with an eight-connection maximum and no busy-timeout pragma, matching the documented custom-handle path but not Harnez telemetry's explicit WAL/busy-timeout setup.

Seven processes succeeded in 22.793–319.447 ms (median 124.410 ms). One process failed during DBOS migrations with `SQLITE_BUSY: database is locked`. Logs from six of the eight processes recorded a busy/locked warning or terminal error, eight occurrences total. DBOS retried some transient errors, but the startup failure was terminal. This is one trial and does not isolate every driver/pragma or migration configuration; repeat with the intended production DSN and owner topology before treating it as a release-level limit.

## Fit with the 825 ledger

| 825 requirement | DBOS contribution | Harnez work still required |
|---|---|---|
| Durable task admission and idempotent retry | Workflow ID provides idempotent identity; queues durably hold workflow work. | Define the caller key, payload conflict behavior, task ownership and API response contract. |
| Checkpoints and restart recovery | Completed steps are checkpointed; pending workflows resume on startup. Canary verified this for one interrupted step. | Choose provider-turn boundaries, version workflow code safely, import existing sessions, quarantine repeatedly failing runs, and define recovery policy for each driver. |
| Leases and fencing | DBOS schedules workflow execution and queues with worker limits. | Show that DBOS's internal ownership is sufficient for Harnez's native CLI process lease and stale-worker fencing. The canary did not test competing workers or fencing tokens; keep Harnez's explicit lease/fencing contract until proven. |
| Timers and schedules | Durable sleep and persisted schedules are available. | Define overdue-run deduplication, time zone/DST policy, quota wakeups and ownership of user schedules. |
| Cancellation trees | Workflow cancellation APIs include child cancellation options. | Propagate cancellation into provider CLI/process groups, persist Harnez's child-agent policy and record partial/uncertain external effects. |
| Durable approvals | Events/messages can let a workflow wait for a decision. | Store requester identity, policy snapshot, authorization, audit trail and approval idempotency in Harnez's domain model. |
| Provider effects and exactly-once | `RunAsTransaction` can atomically couple DB writes and DBOS checkpoints on one handle. | A provider CLI or other external effect cannot join a SQLite transaction. Keep idempotency keys/postcondition checks and mark interrupted unsafe steps uncertain; DBOS does not make those effects exactly once. |
| Client cursor and notification outbox | DBOS exposes workflow events/streams. | Decide whether these satisfy Harnez's durable cursor, snapshot resync and transactional notification outbox semantics; they are not established by this canary. |

For the planned single-owner `harnez serve`, DBOS could own workflow execution while Harnez keeps the provider-turn worker and domain boundary. The SQLite multi-process sample makes the server ownership boundary important: clients and short-lived commands should not each initialize DBOS against the same SQLite file.

## Store-by-store verdict for (b)

| Current store | Recommendation | Reason |
|---|---|---|
| Telemetry SQLite (`internal/telemetry`) | Keep plain SQLite. | It is already a server-designated writer with WAL, `busy_timeout`, idempotent schema setup and an explicit cold-open retry. DBOS workflow history adds no value to ordinary tool-call analytics. |
| Benchmark SQLite (`internal/bench`) | Keep plain SQLite. | This is a separate append/query store for benchmark runs, not a resumable workflow. |
| Subagent session registry JSON (`internal/subagent/session.go`) | Keep files for now. | It stores provider/session identity and operational process metadata in JSON files with file-store semantics; moving it would couple routine CLI session operations to DBOS startup and schema. A future 825 ledger may index/import these records without replacing their provider artifacts. |
| Per-session state, usage JSON/JSONL and snapshots | Keep files. | Small session-local counters, caches and snapshots have low-cost atomic/file-oriented ownership and no workflow replay requirement. DBOS would add a database lifecycle to fresh CLI invocations. |
| Feedback JSONL | Keep append-only JSONL. | It is a simple local event log; no scheduler or replay engine is needed. |
| Future 825 task ledger | Decide with (a), not as a general state migration. | DBOS is worth prototyping only for workflow-shaped server-owned durable tasks. Plain SQLite remains the lower-complexity baseline if task semantics are mostly rows, events, leases and explicit recovery rules. |

Harnez's existing server/ledger design is described in [HarnezServerArchitecture.md](../HarnezServerArchitecture.md); the prior agent-server research and Pi comparisons are in [2026-10-20-harnez-server-durable-research.md](2026-10-20-harnez-server-durable-research.md).

## Alternatives

- **Plain SQLite ledger (825 baseline):** explicit schema, leases, event log and turn-boundary recovery; smallest dependency and easiest to inspect/debug with SQLite tools.
- **Temporal Go SDK:** stronger fit if Harnez later needs a separately operated workflow service and broader worker coordination; introduces a service and deployment model beyond a local SQLite server. See [Temporal Go SDK](https://github.com/temporalio/sdk-go).
- **Other embedded Go workflow libraries:** evaluate only against the same recovery and multi-process SQLite canary; no alternative was benchmarked in M1.

## Canary excerpt

The isolated canary used the SDK's optional SQLite driver and checkpointed each step:

```go
import _ "github.com/dbos-inc/dbos-transact-golang/dbos/driver/sqlite"

ctx, err := dbos.NewContext(context.Background(), dbos.Config{
    AppName:        "dbos-canary",
    SQLiteSystemDB: db,
})
if err != nil { return err }
dbos.RegisterWorkflow(ctx, workflow)
if err := ctx.Launch(); err != nil { return err }

_, err = dbos.RunAsStep(ctx, func(stepCtx context.Context) (string, error) {
    return runOneProviderTurn(stepCtx)
})
```

Full canary source remains in the approved external scratch directory. No DBOS dependency or canary source was added to Harnez.
