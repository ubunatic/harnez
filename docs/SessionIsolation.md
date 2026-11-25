# Session Isolation

Harnez is an execution environment. It does not know who calls it. This doc describes the
current model after runner mode was removed (`859d787b`, 2026-11-25). The runner split from
issue 1181 is closed; its history is in that issue and in git.

## Current model

- **No caller-keyed behaviour.** Harnez does not read `TAIA_SESSION_DIR` or any other caller
  variable, and no code path branches on "runner" versus "host". Non-test Go code has no
  `TAIA_` reads.
- **Quota-1 is advisory.** `harnez exec --quota-1` (and `make test-q1`) records each run and
  reports a repeat run without source changes, then runs it anyway. Quota-1 does not wrap commands
  in bwrap or Landlock and never blocks. The rule is in `.harnez/rules/Quota.md`.
- **Quota-1 advice is gated.** The line "N file(s) changed after the last quota-1 run ... run
  make test-q1" prints only when the repo Makefile has a `test-q1` target and `rules.quota` is
  true in `.harnez/repo.yaml` (`quota1Advised` in `cmd/harnez/agent_run.go`; issue 1287 covers the
  init side).

## Agent store override

Precedence for the session registry and the model access records:

1. `--store-dir <dir>` flag,
2. `HARNEZ_STORE_DIR` when non-blank (`subagent.StoreDirEnv`, `internal/subagent/session.go`),
3. `~/.harnez/agents`.

The override moves the whole store, including model access. With a store set, access records
live in `<store>/models/access.yaml` with `access.yaml.lock` beside it, via
`modelcatalog.SetAccessDir` (set in the root `PersistentPreRunE`). Without it, they live in
`$XDG_STATE_HOME/harnez/models`.

- There is no read fallback. Reads and writes use the same directory, so a command that reads
  records (`harnez agent models`) must get the same `--store-dir` to see them. Decided with the
  taia session, 2026-11-25.
- A test that sets the override and makes the default state dir read-only must see the start
  write nothing outside the store and print no access warning (`agent_access_test.go`).

## Staged installation

- `harnez install --stage <dir>` copies the running binary and writes its man pages into a new
  private destination (`internal/selfinstall/stage.go`). It runs no shell, does not run `go install`,
  does not install the `⚙` alias, and writes no shared location. The destination must be absolute and
  clean, its parent must already exist as a real directory with mode `0700` and no symlink components,
  and the destination itself must not exist. The stage holds `bin/harnez` and `share/man/man1/`. On
  failure the new stage is removed.
- `harnez install --staged <dir>` (`make promote STAGE=<dir>`) promotes a staged build into the host
  GOBIN or GOPATH/bin and the XDG man roots (`internal/selfinstall/promote*.go`). It pins directories
  without following links, requires the exact generated man manifest, verifies SHA-256, creates the `⚙`
  symlink, and renames each file atomically.
- The caller must supply a trusted private parent. The flow does not defeat same-UID path races, and it
  does not protect against arbitrary shell code or a hostile `MAKEFLAGS` set before invocation.
- The promoter roots come from the ownership contract in `spec/session_isolation.yaml`, which is
  tracked in ticket 1289.

## Leftovers (tracked, not yet removed)

- **Ownership contract.** `spec/session_isolation.yaml` and its schema still declare runner, broker
  and shared owners. `internal/sessioncontract` loads them for the runner operation allow-list,
  the bwrap guidance and the promoter roots in `internal/selfinstall/promote.go`. Ticket 1289.
- **Runner-named code.** `internal/server/runner_*.go` and `cmd/harnez/telemetry_server.go` keep
  the inherited-FD broker transport (`SOCK_SEQPACKET` socketpair, `ExtraFiles`). It is a
  transport, not a mode. Two comments in `runner_channel.go` still say "runner mode". Ticket 1289.
- **Dead bwrap probe.** `internal/exec/bwrap_contract.go`: `ProbeBwrapSupport` has no non-test
  caller. Ticket 1289.
- **Telemetry drops stay quiet.** Dropped telemetry prints nothing unless `HARNEZ_DEBUG` or
  `HARNEZ_TELEMETRY_DEBUG` is set (issue 1250).
- **Stray host repository.** `/home/uwe/.git` (only `info/`) exists on the owner's host. Harnez
  no longer writes into it (`FindGitDir` requires `HEAD`, `c6cf2219`), but deleting it is the
  owner's decision.

## Testing against this model

Tests must not depend on `TMPDIR`, `GOBIN`, `GOPATH`, `HOME` or the caller's environment; the
"Hermetic tests" bullet in [Testing.md](Testing.md) lists the rules. Unix socket paths go through
`internal/testutil/socktest`, which keeps them under the 107-byte `sun_path` limit.
