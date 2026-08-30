# 110 — Remote Load: batch-default with an opt-in streaming channel, graceful fallback

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [[051-multi-host-remote-monitoring-and-dashboard-navigation]],
[[090-remote-load-panel-uses-local-metrics]],
[[104-agy-quota-collector-requires-live-process-poll-coincidence]] (same "batch on-demand vs. persistent channel" tradeoff shape, different subsystem),
`internal/usage/remote.go`, `internal/usage/load.go`, `docs/Canary.md`,
`docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md`

## Problem / Goal

Design session 2026-08-30. Two usage-related needs turned out to have different
shapes and should not share one collection path, even though they currently do
(both ride `CollectRemote`'s single SSH call to `harnez usage --json`):

1. **Agent usage/quota data** — occasional, single-host, on-demand. The user works
   on one machine most of the time and occasionally switches. The existing simple
   SSH-loop (`CollectRemote`, one `ssh ... harnez usage --json` call per poll) is
   correct and sufficient here. **No change wanted to this path** — it stays the
   default and the fallback (see below).
2. **CPU/GPU load data** — a different motivation: watching whether a *local LLM
   setup* is running hot on another box, independent of which machine is the
   active usage target. This should be **always-on** for any configured remote
   (not a per-session toggle), and must **merge into the existing single `[L]
   Load` box** alongside local numbers — not a separate box, and not an
   either/or switch between showing local or remote.

Today's gap (per issue 090): the `[L] Load` box shows local CPU/GPU by default,
or — only when `--host` is passed for *usage* purposes — the remote's,
*exclusively*, retitled `Load (@host)`. There is no mode that shows both
simultaneously, and remote Load collection has no independent trigger from
remote usage collection.

## Desired shape: batch (default) + streaming (opt-in), with fallback

- **Batch mode (default, unchanged)**: keep `CollectRemote`'s existing SSH-loop
  exactly as-is for usage. For Load specifically, this becomes a periodic
  poll independent of the usage `--host` flag/cadence — see Acceptance
  Criteria — but the *mechanism* (one-shot SSH call per tick) is the same
  simple approach, just triggered on its own schedule when a remote is
  configured for Load-watching.
- **Streaming mode (new, opt-in)**: a persistent channel dedicated to remote
  CPU/GPU telemetry, pushing samples continuously at a cadence suited to a
  live `--watch` view (faster than a reasonable batch poll interval, without
  paying a fresh SSH handshake per sample).
- **Efficiency once streaming is up**: don't run batch SSH calls *alongside*
  an open stream for the same host — route the occasional batch-shaped fetch
  (e.g. a one-off usage/quota check against that host) through the same
  channel/connection instead of opening a second one. One connection per
  streamed host, not stream + separate periodic batch calls.
- **Resilience**: streaming is opt-in and never the *only* path. If the stream
  dies (network blip, remote reboot, SSH timeout), degrade to the batch
  SSH-loop so Load keeps working at the lower cadence; automatically attempt
  to re-establish/upgrade to streaming once the remote is reachable again —
  no manual restart required from the user.

## Open design question: how do we implement the streaming channel?

This needs a decision before implementation — options below, none yet
chosen. Per `docs/Canary.md` (probe external mechanisms before building
features on them), whichever option is picked should get a canary probe of
the actual mechanism (e.g. a real `ssh -M`/`ControlMaster` round trip against
a real host) before code is built around it.

### Option A — OpenSSH `ControlMaster` connection multiplexing

Open one master connection (`ssh -M -S <control-path> -fN host`), then reuse
it for both a persistent streaming child session (`ssh -S <control-path> host
"harnez load-stream"`, a new remote subcommand that loops sampling
`/proc`/`/sys` and emits one NDJSON line per sample to stdout) and any
on-demand batch command (`ssh -S <control-path> host "harnez usage --json"`).
OpenSSH multiplexes both over one underlying TCP+SSH connection — no new Go
dependency, reuses the exact `exec.Command("ssh", ...)` pattern this codebase
already uses everywhere (`remote.go`, `watch.go`, `history.go`).

- **Pros**: minimal new code, no custom wire protocol, OpenSSH does the hard
  part (connection reuse, keepalive), degrades naturally — if the control
  socket is gone, a plain `ssh` call just reconnects normally (built-in
  fallback behavior per `ssh_config(5)`'s own ControlPath description,
  confirmed locally: `OpenSSH_10.2p1`).
- **Cons**: "batch piped through the stream" is satisfied at the *connection*
  level (shared TCP/SSH transport), not as literally one multiplexed
  application stream — a batch call is still a distinct SSH channel/exec,
  just cheap because the handshake is already amortized. Control-socket
  path management (cleanup, one per host, permissions) is a small but real
  surface.

### Option B — Custom NDJSON multiplexed protocol over one persistent channel

One `ssh host "harnez load-stream"` session, kept open, speaking a small
framed line-protocol over its single stdin/stdout pair: unsolicited
`{"type":"load_sample",...}` lines pushed continuously, plus
request/response pairs (`{"type":"usage_request","id":...}` /
`{"type":"usage_response","id":...,...}`) for on-demand batch fetches
multiplexed onto the same stream.

- **Pros**: literally one channel doing everything, matches the "pipe the
  batch fetch through it" idea most directly, no dependency on OpenSSH
  version/config quirks for multiplexing.
- **Cons**: real protocol design/implementation surface (framing, request
  IDs, backpressure, partial-line handling on a flaky link) — the kind of
  "machinery sized for a need that doesn't exist yet" this project has
  explicitly avoided before (see the SQLite/DuckDB rejection in
  `docs/studies/2026-08-28-usage-collector-daemon-architecture.md` §4, same
  reasoning shape applies here).

### Option C — Naive high-frequency batch polling (no real streaming)

Just poll `CollectRemote` faster. Rejected as a serious option: doesn't meet
the "efficient" requirement — every sample pays a fresh SSH connection setup,
which is the exact cost issue 090 already identified as the expensive part
("one SSH subprocess and round trip is orders of magnitude more expensive
than several procfs/sysfs reads"). Listed here only as the baseline the other
options improve on.

### Option D — TCP/WebSocket/gRPC tunnel

Port-forward (`ssh -L`) or a raw socket, possibly with a new Go dependency
(e.g. `gorilla/websocket`, gRPC). Rejected: heavier than the problem needs,
against `docs/lang/Go.md`'s avoid-deps bias, and duplicates transport/auth
plumbing SSH already provides for free. Not seriously considered further
unless A and B both prove unworkable in canary testing.

**Leaning**: Option A (ControlMaster) as the pragmatic default — smallest new
surface, reuses proven patterns, real fallback behavior for free. Option B
stays on the table if "batch truly interleaved with the live sample stream"
turns out to matter in practice (e.g. sample loss/jitter from spawning a
separate `ssh` child while the control master is busy). Not decided —
flagging both for the canary/scoping pass this ticket is asking for, not
picking one unilaterally.

## Acceptance Criteria (draft — refine during scoping)

1. Remote Load collection has its own trigger/config, independent of the
   usage `--host` flag — e.g. a `load.watch_host` (or similar) key in
   `~/.config/harnez/local.yaml` (issue 109's new local-config layer is the
   natural home for this), separate from `usage.default_host`.
2. When a Load-watch host is configured, the `[L] Load` box always shows
   local **and** that remote's CPU/GPU together in one box — never a
   separate box, never local-only-vs-remote-only as an exclusive choice.
3. Batch mode (current SSH-loop) remains the default and requires no opt-in;
   streaming is explicitly opt-in (a flag or config key).
4. Streaming failure is transparent: on stream death, Load data continues
   arriving via batch polling at a reasonable interval; streaming
   re-establishment is attempted automatically without user action.
5. Whichever streaming mechanism is chosen gets a canary probe (real SSH
   round trip against a real host, not just unit tests against a mock)
   before the full implementation is built, per `docs/Canary.md`.
6. No vendor CLI/SDK telemetry source introduced on either side — same
   kernel-standard-only policy as local collection.

## Explicitly out of scope for this ticket

- Multi-host usage/quota monitoring with a host-switcher TUI (that's
  [[051]] — different goal, not merged into this one).
- Any change to the existing single-host batch usage path (`CollectRemote`),
  which stays exactly as-is per the stated goal above.
