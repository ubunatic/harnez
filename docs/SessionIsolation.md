# Session Local Staging

`bash scripts/stage-install.sh "/absolute/private/path"` builds `harnez` and
its man pages under a new destination. Pass the destination as one quoted
positional argument; there is intentionally no Make staging entry point. The
script currently requires Linux,
Bash, GNU or uutils coreutils `realpath`/`stat`, GNU Make, and Go 1.20 or newer
(for `go build -C`). The destination's parent must already exist,
be a real directory with mode `0700`, and have no symlink components. The
destination itself must not exist. A destination with spaces is supported.

The stage contains `bin/harnez` and `share/man/man1/`. The command does not run
`go install`, install the `⚙` alias, or write man pages to a shared location.
On build or man generation failure it removes the newly created stage. The
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

## Planned ownership contract (M8–M17)

The staging and registry behavior above describes the live M1–M6 implementation.
The following is the approved target, not a claim that M8–M17 have landed. The
machine-readable contract is `spec/session_isolation.yaml`.

| Owner | Paths and operations |
| --- | --- |
| Runner, under `$TAIA_SESSION_DIR` | Session locks and history; exec process records; local usage caches (including live-fetch cache), fetch durations, killlog and passive debug; AGY meter CA, key, bundle, `usage.jsonl` and backoff; `modelblock` and `tier_overrides` overlays; telemetry spool; runner debug/logging, feedback, bench and legacyarchive writes. |
| Collector/broker | Telemetry ingestion and bounded spool replay; canonical usage snapshots and history; own-session registry register, update and query. A trusted launcher passes an inherited connected socketpair/FD bound to the launch identity. The runner channel excludes `/shutdown`, `merge_legacy`, arbitrary paths or PIDs, and cross-session stop/resume. |
| Shared, host-owned or coordinated | `~/.harnez/agents` remains host-owned for discovery and resume. Global `modelblock` and `tier_overrides` are runner read-only, with writes host-only. Runtime shims in `internal/subagent/agy.go` are provisioned by the host. Repository `.harnez` quota and checkout state remain shared coordination. |

The broker must reject forged session identities and cannot rely on same-UID
socket modes or readable bearer tokens for authentication. Runner policy changes
apply only to overlays; no runner RPC mutates global policy.

A sandbox stages privately with `scripts/stage-install.sh`. A trusted host
promoter derives its approved roots only from trusted host configuration and
defaults: `GOBIN` or `GOPATH/bin` for `harnez` and a host-generated relative
`⚙ -> harnez` symlink, and `XDG_DATA_HOME/man/man1` or
`~/.local/share/man/man1` for the exact man-page manifest. It copies and verifies
staged artifacts as immutable host-owned temporary files, pins destination
directories with `O_DIRECTORY|O_NOFOLLOW`, and promotes each approved file
atomically. Atomicity is per file, not across the whole set. Runner-supplied
arbitrary or checkout destinations are not approved.

These controls do not provide read confidentiality when `/` is readable under
Taia, shared-checkout isolation, a same-UID filesystem boundary, or trust in
arbitrary code. Taia's external enforcement and supported host environment remain
prerequisites for cross-runner denial claims.
