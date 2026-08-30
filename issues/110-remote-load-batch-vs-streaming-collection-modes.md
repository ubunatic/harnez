# 110 — Remote Load: batch-default with an opt-in streaming channel, graceful fallback

**Status**: Open — design fully decided (mechanism, config, lifecycle, v1 rendering) and canary-verified against a real host; implementation not yet started
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [[051-multi-host-remote-monitoring-and-dashboard-navigation]],
[[090-remote-load-panel-uses-local-metrics]],
[[104-agy-quota-collector-requires-live-process-poll-coincidence]] (same "batch on-demand vs. persistent channel" tradeoff shape, different subsystem),
[[109-user-local-config-xdg-config-harnez-local-yaml]] (local config layer this ticket's `load.watch_host` key lives in),
`internal/usage/remote.go`, `internal/usage/load.go`, `internal/usage/watch.go`, `docs/Canary.md`,
`docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md`

## Problem / Goal

Design session 2026-08-30. Two usage-related needs turned out to have different
shapes and should not share one collection path, even though they currently do
(both ride `CollectRemote`'s single SSH call to `harnez usage --json`):

1. **Agent usage/quota data** — occasional, single-host, on-demand. The
   primary use case is a single workstation running `harnez usage --watch` in
   a small, permanently-open terminal split (e.g. a Tilix pane), showing
   token/quota usage across all locally-running agents. Occasionally the
   user starts an agent (and `harnez usage`) on a different host; whether
   that host has network access back to the workstation isn't guaranteed and
   isn't the main scenario — this secondary case needs basic functionality
   only, not full design treatment. The existing simple SSH-loop
   (`CollectRemote`, one `ssh ... harnez usage --json` call per poll) is
   correct and sufficient here. **No change wanted to this path** — it stays
   the default and the fallback (see below).
2. **CPU/GPU load data** — a different motivation: watching whether a *local
   LLM setup* is running hot on another box, independent of which machine is
   the active usage target. This wants to be baked into the same `harnez`
   tool (not a separate dashboard/tool) and configured once, separately from
   the occasional `usage --host` switch.

Today's gap (per issue 090): the `[L] Load` box shows local CPU/GPU by
default, or — only when `--host` is passed for *usage* purposes — the
remote's, *exclusively*, retitled `Load (@host)`. Remote Load collection has
no independent trigger from remote usage collection, and there's no way to
see local and a remote's load at the same time.

## Decisions (2026-08-30)

### 1. Trigger/config: a new, independent `load.watch_host` key

Lives in `~/.config/harnez/local.yaml` (issue 109's local-config layer),
alongside but separate from `usage.default_host` — the two are not
necessarily the same machine (occasional ad-hoc `--host` usage vs. a
specific box you always want Load telemetry from).

```yaml
usage:
  default_host: um760       # existing (issue 109)
load:
  watch_host: llm-box        # new (this ticket) — independent of usage.default_host
```

The remote Load box (see §5) only appears when `load.watch_host` is set;
absence is the same as today (local-only Load box). Applies uniformly to
`--watch` and one-shot modes (`--summary`, plain print) — see §2 for how
each mode actually fetches the data.

### 2. Streaming is `--watch`-only; other modes always use plain batch

- **`harnez usage --watch`**: uses streaming (Option A, below) when
  possible, silently falling back to batch polling if streaming can't be
  established or drops mid-session (stale remote binary, network blip,
  etc.) — an internal implementation detail, not user-facing config or a
  flag.
- **Every other invocation** (`--summary`, plain one-shot `harnez usage`):
  always plain batch — a single `ssh host "harnez usage --json"`-style call
  for that tick, no `ControlMaster`, no persistence. There's no long-lived
  process for a stream to live in, and no need to build one for a command
  that prints once and exits.
- This resolves the earlier open question about whether batch mode should
  also opportunistically use `ControlMaster`: it should not. Simpler and
  safer to keep one-shot invocations fully stateless.

### 3. Wire schema: reuse `LoadSnapshot`, no redesign

Each streamed sample is one JSON-marshaled `LoadSnapshot` (`internal/usage/types.go`
— already used for the batch JSON payload since issue 090) per NDJSON line.
No new type, no schema redesign — the existing struct already round-trips
through JSON cleanly for exactly this shape of data. Revisit only if
something concrete surfaces during implementation that makes it
incompatible; no separate prep ticket is warranted up front.

### 4. `ControlMaster` lifecycle is 1:1 with the `--watch` process

Owned entirely by the `harnez usage --watch` invocation that uses it:
started when that process starts (if `load.watch_host` is configured and
streaming is being attempted), torn down on exit — including Ctrl-C — via
the same cleanup path `watch.go` already uses to restore terminal state
(`stty` restoration, etc.). No cross-invocation persistence, no daemon, no
control master ever exists when no `--watch` client is running. This also
answers the earlier "does batch mode benefit from ControlMaster too"
question in the same direction as §2: since the master's lifetime is tied
to one `--watch` process, a separate one-shot invocation has nothing to
reuse anyway.

### 5. v1 rendering: a separate new Load box for the remote host

Not merged into the existing local `[L] Load` box. A distinct box (e.g.
`[R] Remote Load (@host)` or similar — exact key/title TBD at
implementation time) shown alongside it when `load.watch_host` is
configured. Simpler first iteration, avoids a `buildLoadBox` layout
redesign up front. Merging into one unified local+remote box is explicitly
deferred to a later design pass once the basic mechanism is proven.

### 6. Streaming sample cadence

Match the existing local Load box's redraw/sample cadence — no new
interval invented for this ticket.

## Mechanism decision: Option A — OpenSSH `ControlMaster` multiplexing

Chosen and canary-verified against the real remote host (`um760`) before
any feature code — see `scripts/canary-remote-load-stream.sh` (kept per
`docs/Canary.md`'s "keep the canary" rule; run it directly with
`scripts/canary-remote-load-stream.sh <host>`).

**Findings**:
- A cold `ssh` call (fresh handshake, no multiplexing) took ~0.5s.
- A call reusing an already-open `ControlMaster` took ~20-220ms across
  repeated runs — consistently and substantially faster, confirming the
  handshake is the dominant cost, matching issue 090's original assumption.
- A long-lived "stream" child (`ssh -S <ctl> host "for i in 1 2 3; do echo
  sample-$i; sleep 0.2; done"`) and a concurrent "batch" call
  (`ssh -S <ctl> host "echo concurrent-batch-call"`) both ran correctly over
  the same master at once — output interleaved cleanly, neither blocked the
  other.
- After `ssh -S <ctl> -O exit host` tears the master down, a plain `ssh`
  call to the same host still succeeds immediately via a normal
  (non-multiplexed) connection — the fallback-to-batch behavior this ticket
  needs comes for free from `ssh`'s own semantics.
- A separate ad-hoc measurement (HTTP over an SSH port-forward vs. `ssh -S`
  exec) found the real overhead difference between mechanisms is small once
  you stop conflating it with per-call process-spawn cost — see the session
  discussion; not written up as a formal finding since it didn't change the
  decision.

**Implication for implementation**: harnez needs a small `ControlMaster`
lifecycle wrapper (start/check/exit against a per-host control socket path,
owned by the `--watch` process per Decision §4) plus a new remote `harnez
load-stream` subcommand that loops sampling `/proc`/`/sys` (same technique
as `CurrentCPULoad`/`CurrentGPUs` in `load.go`) and emits one `LoadSnapshot`
JSON line per sample to stdout. No custom multiplexing protocol (Option B)
and no new dependency (Option D). Local orchestration stays
`exec.Command("ssh", ...)`, consistent with `remote.go`/`watch.go`/`history.go`'s
existing pattern.

**Deployment dependency**: `harnez load-stream` is a new subcommand, so
streaming only works once the remote's installed `harnez` is rebuilt from a
source tree that has it. `Makefile:92`'s existing `sync` target already
covers this — `git push` locally, then over SSH: `git pull && make install
&& make status` on the remote — no new deploy mechanism is needed. This
also means a remote running a `harnez` predating `load-stream` simply fails
that exec with "unknown command," indistinguishable from any other
stream-unavailable case and already covered by the generic fallback-to-batch
path from Decision §2 — no special-casing an out-of-date remote is required.

Options B/C/D below are kept for context on why A was chosen, not as live
alternatives pending further work.

### Option A — OpenSSH `ControlMaster` connection multiplexing

Open one master connection (`ssh -M -S <control-path> -fN host`), then reuse
it for both a persistent streaming child session (`ssh -S <control-path> host
"harnez load-stream"`) and any on-demand batch command
(`ssh -S <control-path> host "harnez usage --json"`). OpenSSH multiplexes
both over one underlying TCP+SSH connection — no new Go dependency, reuses
the exact `exec.Command("ssh", ...)` pattern this codebase already uses
everywhere.

- **Pros**: minimal new code, no custom wire protocol, OpenSSH does the hard
  part (connection reuse, keepalive), degrades naturally — if the control
  socket is gone, a plain `ssh` call just reconnects normally (confirmed
  locally: `OpenSSH_10.2p1`).
- **Cons**: "batch piped through the stream" is satisfied at the *connection*
  level (shared TCP/SSH transport), not as literally one multiplexed
  application stream. Control-socket path management (cleanup, one per
  host, permissions) is a small but real surface — mitigated by Decision §4
  tying its lifetime to the `--watch` process.

### Option B — Custom NDJSON multiplexed protocol over one persistent channel

One `ssh host "harnez load-stream"` session, kept open, speaking a small
framed line-protocol over its single stdin/stdout pair: unsolicited
`{"type":"load_sample",...}` lines pushed continuously, plus
request/response pairs multiplexed onto the same stream for on-demand batch
fetches.

- **Pros**: literally one channel doing everything, no dependency on OpenSSH
  version/config quirks for multiplexing.
- **Cons**: real protocol design/implementation surface (framing, request
  IDs, backpressure, partial-line handling on a flaky link) — the kind of
  "machinery sized for a need that doesn't exist yet" this project has
  explicitly avoided before (see the SQLite/DuckDB rejection in
  `docs/studies/2026-08-28-usage-collector-daemon-architecture.md` §4).
  Moot now that Decision §2 confines batch calls to their own separate
  one-shot invocations rather than needing to interleave with an open
  stream.

### Option C — Naive high-frequency batch polling (no real streaming)

Just poll `CollectRemote` faster. Rejected: doesn't meet the efficiency
goal — every sample pays a fresh SSH connection setup, the exact cost issue
090 already identified as the expensive part. Listed only as the baseline
the other options improve on.

### Option D — TCP/WebSocket/gRPC tunnel

Port-forward or a raw socket, possibly with a new Go dependency. Rejected:
heavier than the problem needs, against `docs/lang/Go.md`'s avoid-deps bias,
duplicates transport/auth plumbing SSH already provides for free.

## Acceptance Criteria

1. `load.watch_host` in `~/.config/harnez/local.yaml` (`load:` section,
   separate from `usage:`) controls whether a remote Load box is shown at
   all; absence means today's local-only behavior, unchanged.
2. When configured, a **separate, new** Load box for `load.watch_host`
   appears in `--watch`, `--summary`, and plain one-shot output, alongside
   the existing local `[L] Load` box (v1: not merged — see Decision §5).
3. In `--watch`, that box is populated via the streaming mechanism (Option
   A) when available; every other mode uses one plain batch SSH call per
   invocation (Decision §2). Streaming failure in `--watch` falls back to
   batch polling transparently and retries streaming automatically, with no
   user-facing indication required beyond normal staleness/error handling
   already used elsewhere in the usage UI.
4. The `ControlMaster` (when used) is started and torn down entirely within
   the `--watch` process's own lifecycle (Decision §4) — verified by
   confirming no `ssh -M` control socket or remote `harnez load-stream`
   process survives after a `--watch` session exits (including via Ctrl-C).
5. ~~Whichever streaming mechanism is chosen gets a canary probe...~~ Done —
   see the Mechanism Decision section above and
   `scripts/canary-remote-load-stream.sh`.
6. No vendor CLI/SDK telemetry source introduced on either side — same
   kernel-standard-only policy as local collection.

## Explicitly out of scope for this ticket

- Multi-host usage/quota monitoring with a host-switcher TUI (that's
  [[051]] — different goal, not merged into this one).
- Any change to the existing single-host batch usage path (`CollectRemote`),
  which stays exactly as-is per the stated goal above.
- Merging the local and remote Load data into one unified box (Decision §5
  defers this to a later design iteration).
