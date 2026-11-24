# Session Local Staging

`make build && ./harnez install --stage /absolute/private/path` copies the running
binary and writes its man pages under a new destination (`internal/selfinstall/stage.go`;
it replaced `scripts/stage-install.sh` in issue 1254). It runs no shell and needs no
coreutils, so any destination name is passed through unchanged. It is allowed in runner
mode; there the man directory must lie inside `$TAIA_SESSION_DIR`, as for `man --dir`.
The destination must be absolute and clean, its parent must already exist, be a real
directory with mode `0700`, and have no symlink components, and the destination itself
must not exist.

The stage contains `bin/harnez` and `share/man/man1/`. The command does not run
`go install`, install the `⚙` alias, or write man pages to a shared location.
On copy or man generation failure it removes the newly created stage. The
caller must provide a trusted private parent, such as a session workspace
created by its host. This workflow does not install into shared destinations
safely: it does not provide a trusted installer or defeat same-UID path races.

The session registry remains at its existing shared default unless the caller
supplies `--store-dir`. Changing that default would currently hide runner
records from host discovery and resume because hosts do not have a contract for
discovering runner-local stores. Workers inherit `TAIA_SESSION_DIR` and nested
workers receive an explicit `--store-dir`; this is not a complete registry
discovery contract or a same-UID security boundary. Invalid runner state must
not silently fall back to shared state.

The stage test uses fake Go and binary commands to check path validation,
failure cleanup and exit status, and that hostile `GOBIN`, `GOPATH`, `HOME`,
and `XDG_DATA_HOME` values do not redirect writes. It invokes the script
directly with quotes, backticks and literal command-substitution text in the
quoted destination, verifies no marker was created, and checks shared binary,
alias and man directories retain only regular-file sentinels, including the
`⚙` alias location. It tests skip behavior for unsupported platforms and
coreutils; `make test` prints a `SKIP` message and continues there. The direct
installer rejects unsupported Linux/coreutils requirements with an error.
The caller's shell and environment must be trusted; this workflow
does not protect against arbitrary shell code or hostile `MAKEFLAGS` processed
before invocation. A real
staged binary should also be checked for generated man pages. Taia's private directory creation and
Landlock policy are external assumptions; Harnez cannot prove cross-runner
denial from these tests.

## Ownership contract and remaining work

The staging and registry behavior above describes the live M1–M6 implementation,
and M8–M10b runner isolation is now active under `$TAIA_SESSION_DIR` and the
inherited broker FD (`HARNEZ_BROKER_FD`):

- Host launchers (`internal/exec`, `cmd/harnez/subagent.go`,
  `internal/subagent/{process,codex,interactive}.go`) attach an `AF_UNIX`
  `SOCK_SEQPACKET` broker socketpair via `ExtraFiles` and run
  `RelayRunnerBroker` for the child lifetime (even for host-launched children
  without `TAIA_SESSION_DIR`, which keeps child-to-collector telemetry ready if
  later wrapped). Runners open ephemeral `SOCK_STREAM`
  requests atomically over `HARNEZ_BROKER_FD` via `SCM_RIGHTS`; only allowlisted
  telemetry writes (`telemetry_ingest`, `telemetry_replay`) pass through to
  `Config.Socket()`, while `/shutdown`, `merge_legacy`, and `ensure_schema`
  fail with `403`. Validated inherited FDs are marked close-on-exec at startup
  so non-broker subprocesses cannot inherit the descriptor.
- Direct `bwrap` execution preserves `ExtraFiles` (`HARNEZ_BROKER_FD` survives
  when explicitly forwarded), whereas external `taia` sandbox helpers spawn
  child commands without forwarding FDs `>= 3` (`cmd.ExtraFiles` empty /
  `UnixRights(0,1,2)`). When the broker FD is missing, invalid, or stripped
  across an exec boundary, Harnez fails closed without falling back to
  `Config.Socket()`.
- **Diskless broker telemetry and quiet drops (#1250)**: `server.DispatchTelemetry`
  (`internal/server/runner_channel.go`) routes over an inherited `HARNEZ_BROKER_FD`
  even when `TAIA_SESSION_DIR` is unset, and never touches host spool paths on
  disk. When telemetry cannot be delivered, `sendTelemetryRecord` suppresses
  stderr drop warnings by default (enable with `HARNEZ_DEBUG` or
  `HARNEZ_TELEMETRY_DEBUG`) so confined CLI calls stay noise-free.
- **Read-only runner registry inspection and actionable errors (#1270, #1279)**:
  In runner mode, `harnez agent start --dry-run` skips opening the host session
  store, and `agent list`/`status`/`wait` read `~/.harnez/agents` through
  `OpenSessionStore` (`readOnly: true`, failing closed across all write/lock
  methods and skipping stale-session repair in `waitForAgent`). Mutating
  `NewSessionStore` and host-only commands (`start`, `resume`, `stop`, `delete`)
  stay guarded and name `mcp__taia__background_task` in their errors (note:
  drivers whose client-tool path does not expose Taia plugin tools, such as
  `agy`, cannot invoke `mcp__taia__background_task` directly).
- Under Taia confinement, Landlock policy is the security boundary blocking
  direct access to `Config.Socket()` and `~/.harnez/agents`; in-process
  `TAIA_SESSION_DIR` checks are defense in depth for honest runner code, while
  the host broker enforces cryptographic per-launch `LaunchID` and host-bound
  process tree validation on `own_register`, `own_update`, and `own_query`.
  Records created directly on the host intentionally carry a distinct `LaunchID`
  (or none for legacy files) and cannot be adopted or mutated by a runner.

The matrix below is the full M7–M17 contract (`spec/session_isolation.yaml`),
covering live Landlock confinement, the `bwrap` prerequisite profile, and host
promotion.

## Bubblewrap prerequisites & migration contract

- **Quota-1 is not sandboxed**: `harnez exec --quota-1` no longer wraps commands in `bwrap`
  or the Landlock fallback, and it no longer blocks runs. A repeat run without source changes
  prints an advisory note and runs anyway. Quota-1 is a recommendation (`.harnez/rules/Quota.md`).
- **Taia `--allow-bwrap` boundary & host prerequisites**: `ProbeBwrapSupport`
  and `DiagnoseBwrapError` report explicit diagnostics for
  `Failed to make / slave: Operation not permitted`, `setting up uid map:
  Permission denied`, `Creating new namespace failed`, missing `bwrap`, or
  non-Linux hosts. Running `harnez exec` inside Taia Landlock confinement without
  `--allow-bwrap` no longer matters for `harnez exec`, because Quota-1 does not
  wrap commands. `--allow-bwrap` is needed only when `bwrap` itself is used.
- **Pre-M12 legacy session migration**: Host files in `~/.harnez/agents/*.json`
  created before `LaunchID` binding carry an empty `launch_id`. No bulk
  backfill migrates legacy sessions into runner control; they remain readable on
  the host and intentionally fail closed (`403`) on runner-broker `own_update`
  / `own_query` / conflict checks.

| Owner | Paths and operations |
| --- | --- |
| Runner, under `$TAIA_SESSION_DIR` | Session locks and history; exec process records; local usage caches and live-fetch cache, fetch durations, killlog and passive debug; AGY meter CA, key, bundle, `usage.jsonl` and backoff; `modelblock` and `tier_overrides` overlays; telemetry spool; runner debug/logging, feedback, bench and legacyarchive writes. |
| Collector/broker | Telemetry ingestion and bounded spool replay; host-owned canonical usage snapshots and history; own-session registry register, update and query. A trusted launcher passes an inherited connected socketpair/FD bound to the launch identity. The runner channel excludes `/shutdown`, `merge_legacy`, arbitrary paths or PIDs, and cross-session stop/resume. |
| Shared, host-owned or coordinated | `~/.harnez/agents` remains host-owned for discovery and resume. Global `modelblock` and `tier_overrides` are runner read-only, with writes host-only. Runtime shims in `internal/subagent/agy.go` are provisioned by the host. Repository `.harnez` quota and checkout state remain shared coordination. |

The broker must reject forged session identities and cannot rely on same-UID
socket modes or readable bearer tokens for authentication. Runner policy changes
apply only to overlays; no runner RPC mutates global policy.

A sandbox stages privately with `harnez install --stage <dir>`. A trusted host
promoter (`harnez install --staged <dir> [--sha256 <hex>]`, or `make promote
STAGE=<dir>`) derives its approved roots only from trusted host configuration
and defaults: `GOBIN` or `GOPATH/bin` for `harnez` and a host-generated relative
`⚙ -> harnez` symlink, and `XDG_DATA_HOME/man/man1` or
`~/.local/share/man/man1` for the exact man-page manifest of the running host
binary. It copies staged artifacts into fsynced host-owned temporary files,
checks file/dir ownership, rejects FIFOs/hardlinks/symlinks, checks `--sha256`
when provided, pins destination directories with `O_DIRECTORY|O_NOFOLLOW`, and
promotes each approved file atomically. Atomicity is per file, not across the
whole set; without `--sha256`, the copy digest is only a structural I/O check
and relies on host same-UID trust rather than cryptographic build signing.
Runner-supplied arbitrary or checkout destinations are not approved; Landlock
blocks shared destination writes at the kernel boundary while `TAIA_SESSION_DIR`
checks fail closed early against `goself`, `install <dep>`, and shared `man
--install` calls.

On the host, `make install` runs `./harnez install --strategy self` (`CGO_ENABLED=0`,
`goself v0.5.0`, #1245 / PR #222) into `~/.local/bin/harnez`, warns on `PATH`
shadows, and atomically symlinks `$GOBIN/harnez -> ~/.local/bin/harnez`
(`ensureGoBinSymlink` in `internal/selfinstall/install.go`) so interactive host
shells and Taia `bash -lc` background tasks invoke the same binary. Aligning the
host promoter's approved binary root (`GOBIN`/`GOPATH/bin`, A4) with
`~/.local/bin` remains an open decision in issue #1258.

These controls do not provide read confidentiality when `/` is readable under
Taia, shared-checkout isolation, a same-UID filesystem boundary, or trust in
arbitrary code. Taia's external enforcement and supported host environment remain
prerequisites for cross-runner denial claims.
