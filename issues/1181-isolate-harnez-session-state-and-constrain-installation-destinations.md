# 1181 — Isolate Harnez session state and constrain installation destinations

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Architecture

## Goal

/goal Enable least-privilege Harnez operation across concurrent agent sessions and narrowly scoped installation, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, record what was tried, why it failed, and the resume point.

## Problem and acceptance

Agents currently need access to shared `~/.harnez/sessions/` and `/run/user/$UID/harnez/procs/`, and `make install` writes shared executable and man-page directories. Broad write access risks tampering with other sessions and replacing unrelated binaries. Confirm the actual live layout, permission requirements and recent history before implementation.

- Design and verify session-scoped state, locks, logs and process records so one sandboxed session cannot alter another's; use a trusted broker for any necessary shared registry. Do not rely solely on same-user filesystem modes for isolation.
- Provide or document a constrained installation flow: build in a session workspace and install only explicitly approved Harnez binary and man-page destinations, without granting agents write access to all of `~/go/bin` or the shared man directory. Address symlink and path substitution attacks.
- Document exact read/write needs and coordination with taia's sandbox; test cross-session denial and allowed normal operations. Assess compatibility and migration rather than assuming a particular layout.
- Investigate separately the observed Bubblewrap `setting up uid map: Permission denied`; filesystem allowlists alone cannot fix namespace policy.

## Milestone 2 — Runner state validation refinements

Independent review of e437a41d identified three refinements. This milestone adds a validated default state-directory API for real state writers in the session tip, Claude read hook, exec hook, and edit-guard feedback paths; ensures a supplied environment session ID initializes private runner state before writing; and resolves runner process records without consulting legacy HOME/XDG paths. Regression coverage exercises those writer paths, invalid runner roots, legacy process paths, and HOME/XDG being unset. Explicit directory overrides and legacy behavior when `TAIA_SESSION_DIR` is absent remain supported. These checks do not establish a same-UID security boundary. Quota-1 did not start: the wrapper found a prior incomplete run with no live process group. Stale tracker state was released, and this milestone was not rerun. `make install` succeeded. External sandbox denial tests, constrained installation, and the remaining M1 acceptance work remain open.

## Milestone 3 — Preserve read denials when state is unavailable

Independent review of 052b4e65 found that an invalid runner state path could return from the read hook before emitting an already-computed denial. The hook now skips repeat-read state tracking when its default state path is unavailable while still returning the computed policy denial. A regression uses a prohibited 100-line read and an invalid runner root, and checks the serialized denial response. `make test-q1` again stopped before tests because bwrap failed to make `/` a slave with `Operation not permitted`; no sandbox bypass was attempted. `make install` succeeded. Cross-session sandbox denial, constrained installation and remaining acceptance work are open.

## Next milestone — staging review pre-work

Independent review of d56189ab: prevent Make recipe shell interpretation of STAGE_DIR (test literal quotes, backticks and substitutions); integrate staging tests into normal verification; assert failure injection reaches build and man generation and verify complete shared destination contents including aliases. Address GNU-only realpath/stat portability or explicitly detect and document the supported platform. Preserve trusted-parent scope and do not claim race-safe shared installation. Owner reported `make test` PASS at d106b961; the staging milestone's Quota-1 run failed before tests due to Bubblewrap.

## Next milestone — staging portability pre-work

Independent review of d106b961..f83c5b0e found that the Linux-only staging test is unconditionally part of make check/test and would break macOS verification; skip it clearly on unsupported platforms while retaining explicit installer rejection. Strengthen the shared alias assertion to verify GOBIN/⚙ content and type, not merely its name. Previous direct-script injection and failure reachability findings were addressed; full suite remains blocked before execution by Bubblewrap.

## Next milestone — hidden-file assertion pre-work

Review of 94e4b306 found that shared-directory completeness checks use a glob that omits dotfiles. Strengthen the assertion and add a regression proving an unexpected hidden entry is detected; preserve portability skip behavior and all existing sentinel checks.

## Session handoff

M1–M3 landed locally through e891dfbc; independent M3 review found no blocking issues. No full suite ran: `make test-q1` failed before tests with `bwrap: Failed to make / slave: Operation not permitted`, including under taia `--allow-bwrap` with /dev and /tmp writable. Resume by diagnosing taia's Bubblewrap broker or running the required suite from a suitable host environment, then review and test the changes before pushing. Constrained installation, broader shared-state handling, and externally enforced cross-session denial tests remain open. Ticket 1183 tracks owner-authorized temporary Quota-1 suspension. Do not claim same-UID isolation.

Host `make test` later reached the suite and reported `TestRecordDirUsesRunnerRuntimeForWriterAndCleanup` failing because `CleanProcs` returned `would-remove` while the test expected `removed`. `CleanOptions.Kill` defaults to false (dry-run); this regression is fixed by setting `Kill: true` in the test, which still asserts actual record removal. No other failures were reported. Host rerun on 2026-11-07 at d106b961: `make test` passed (exit 0, 74 packages ok, no failures), including internal/runstate, internal/procs, internal/agent and internal/githooks.

## Initial bounded milestone: current contracts and observations

## Owner decision and milestone 1: Taia runner-scoped defaults

Taia supplies the sandbox boundary through external Landlock enforcement. Harnez provides best-effort runner-scoped state when `TAIA_SESSION_DIR` is present; filesystem modes and path checks are defense in depth and do not provide a same-UID security guarantee. `TAIA_SESSION_DIR` identifies a private runner directory, not a persistent chat identity. With the variable absent, Harnez keeps its legacy paths. Existing explicit directory overrides retain precedence. Invalid runner roots must fail closed rather than silently use shared defaults.

Milestone 1 routes implicit session lock, ticket history, and consumers of the default session-state directory to `$TAIA_SESSION_DIR/state/sessions`; explicit lock/state directory overrides keep precedence. Exec process records and cleanup use `$TAIA_SESSION_DIR/runtime/procs`. The runner and derived directories are validated by implicit session/ticket resolution and process-record lookup before use. This does not route the agent registry/logs, telemetry, or quota state; it does not constrain installation or prove cross-runner denial. Those remain acceptance work, along with sandbox-backed allow/deny tests, migration/compatibility review, symlink/path-substitution analysis for installation, and separate investigation of the Bubblewrap uid-map denial.

The Taia directory contract is external to Harnez. Tests can verify path selection, rejection of malformed roots, explicit override precedence, and writer/cleanup path agreement; they cannot establish the external Landlock policy or a security boundary between same-UID processes.

The source-level contracts below describe the legacy defaults when `TAIA_SESSION_DIR` is absent, not a host inspection or proof of isolation. Paths depend on the process environment and OS.

- Session resolution uses explicit/environment IDs first; its fallback reads, creates and updates PPID-derived lock files under `$HOME/.harnez/sessions` (directory requested as 0755, lock files 0644, subject to umask). Ticket history and session state also use this directory; session state writes JSON with requested mode 0644. These modes do not isolate processes sharing a UID, and existing paths retain their existing permissions.
- Agent registry defaults to `$HOME/.harnez/agents`, configurable with `--store-dir`. It creates the directory with requested mode 0700, serializes registry operations with `.registry.lock` and flock, and writes session JSON via a temporary file and rename. Async stdout/stderr logs also live in the store. Locks and atomic rename coordinate cooperating writers; they do not authenticate writers or enforce cross-session authorization.
- Exec process records are `$XDG_RUNTIME_DIR/harnez/procs/<pgid>.json` when that variable is set, otherwise `$HOME/.harnez/run/procs/<pgid>.json`. Creation requests 0700 for directories and 0600 for files; existing paths are not secured by those requests. Writers create/overwrite records and cleanup reads and can remove them. Linux cleanup relies on readable `/proc` process identity and UID data, and kernel signal permissions; other hosts or restricted `/proc` may not supply these assumptions.
- `make install` builds `./harnez`, runs `go install` (destination follows Go's `GOBIN`/`GOPATH` rules), generates the command tree's man pages and creates or replaces the `⚙` symlink at `$(go env GOPATH)/bin/⚙`; it falls back to `$HOME/go/bin/⚙` if that command fails. The Makefile uses the full, unsplit `go env GOPATH` output as one path. Man pages go to `/usr/local/share/man/man1` as root, otherwise `$XDG_DATA_HOME/man/man1` or `$HOME/.local/share/man/man1`, with a system fallback. `harnez man --dir` selects another directory. This is not an allowlisted installer; destination directories and path components must be trusted against symlink and rename substitution. A workspace build or custom destination alone does not prove safe installation into shared paths.

## Process safety characterization and options for review

- Cleanup defaults to reporting; `--kill` enables signals. For visible group members it requires known matching UID, checks the leader start time when present, and re-reads group members before each signal. It treats a matching same-UID owner as alive (zero owner start time matches any start time). A stopped group or dead owner is eligible; a zero recorded leader start time is not signalled. A group with no visible members is retained as unknown if the kernel says it exists. Checks and signals are separate operations, and a missing leader is not authenticated by a leader check.
- `TestCleanProcsForgedRecordCanSignal` supplies a forged record with the correct fake leader start time and a dead fake owner. Cleanup issues fake SIGTERM. This passing characterization demonstrates the absence of record authentication, not protection against forgery; no real process is signalled.
- Agent stop builds process, provider and worker roots from the stored session and records the caller's process start time for new sessions. `StopTree` now rejects the whole root set before scanning or signalling if any target PID greater than 1 has an unknown start time; unknown worker roots are retained so a mixed known/unknown set fails closed. Active interactive stops validate the recorded process or caller identity before sending control. This is a limited mitigation: legacy sessions with missing identities and hosts where process start times are unavailable fail closed. PID checks and signals are not atomic, so PID reuse can still race with a signal. Visibility and UID checks do not authorize a writable session record, and a start-time check cannot authenticate a forged record.
- The selected limited mitigation is to reject unknown start times for agent-stop targets before messages or signals. This does not change cleanup behavior, authenticate records, or eliminate scan-to-signal races. For installation, consider a trusted installer that validates fixed approved destination names and directory handles, rejects symlink/path substitution, and installs a workspace-built artifact; define who approves and owns those directories before design.

## Milestone 4 — Session-local staging

Decision: preserve the shared agent registry default until host discovery and resume have a defined contract for runner-local stores. A runner-scoped default would currently hide records from host discovery/resume; scanning sibling runner directories is not an acceptable substitute. Explicit `--store-dir` remains authoritative. This decision does not establish a same-UID security boundary.

`bash scripts/stage-install.sh "<new-absolute-path>"` builds the binary and man pages under a new destination whose existing parent is a real mode-0700 directory without symlink components. The Make entry point was removed because Make can process variable expressions before its Makefile safeguards; a quoted positional argument avoids routing the destination through Make expansion. It does not call `go install`, create the `⚙` alias, or write shared man paths. Build/man failures remove the partial destination. This is session-local staging, not safe shared installation: the caller must provide a trusted private parent, shell, and environment, and the flow does not defeat same-UID path races or protect against arbitrary shell code or hostile `MAKEFLAGS` processed before invocation. See `docs/SessionIsolation.md`.

Validation covers hostile `GOBIN`, `GOPATH`, `HOME`, and `XDG_DATA_HOME`, spaces, relative/noncanonical/existing destinations, symlink and permissive parents, sentinels, and build/man failure cleanup. A real staged build checks man generation. Taia directory provisioning and Landlock denial remain external assumptions. Bubblewrap namespace failure and cross-runner denial remain open.

## Milestone 5 — Staging review fixes

Make expands a malicious `MAKEFLAGS` assignment before parsing the Makefile, so a Makefile origin guard cannot enforce a hostile-MAKEFLAGS boundary; the staging Make entry point was therefore removed. Use the direct Bash script with one quoted destination argument. Direct-script regression tests include spaces, quotes, backticks and literal command-substitution text, verify no marker is created, and run through the ordinary `make test` verification path. Failure injections assert that build/man commands were reached and preserve their exact exit statuses; hostile shared binary, alias and man destinations are checked for unchanged complete contents. The script explicitly requires Linux with GNU or uutils coreutils `realpath` and `stat`, Bash and Go 1.20+; portability beyond that is not claimed. Caller shell/environment and the private parent are trusted; this does not protect against arbitrary hostile shell code or MAKEFLAGS processed before invocation. Same-UID race limitations remain. No claim of race-safe shared installation is made.

## Milestone 6 — Portable staging test integration

The staging test now exits successfully with an explicit `SKIP` reason when Linux or the required GNU/uutils `realpath` and `stat` tools are unavailable, so the normal `make check`/`make test` path remains usable on unsupported platforms. Test-only overrides exercise both skip branches on Linux. The direct installer still rejects unsupported platform/tool combinations; regression checks assert the status and diagnostic. Shared destination checks include dotfiles, verify `GOBIN/⚙` and other sentinels remain regular files with unchanged contents, and prove an unexpected hidden entry is detected. Linux staging coverage remains active on supported hosts.

## Decision

Owner-confirmed architectural choices for M7+ (`2026-11-16` lean-sprint):
- **Broker authority (`A1`)**: Runners authenticate to shared collector/registry/usage RPCs over an inherited connected `socketpair`/FD passed by the trusted host launcher. Do not expose host-only admin routes (`/shutdown`, `merge_legacy`, arbitrary caller paths/PIDs) on the runner channel. Same-UID `0600` sockets and bearer tokens in readable locations are insufficient when `/` is readable under Taia.
- **Global policy (`A2`)**: Global `modelblock` and `tier_overrides` remain read-only shared policy for runners; runner-local changes write to runner overlays under `$TAIA_SESSION_DIR`. Global policy mutations stay host-only (no runner policy-mutation RPC).
- **Agent registry & lifecycle (`A3`)**: Runners register, update, and query only their own launch-bound record via the broker (`~/.harnez/agents` stays host-owned for discovery/resume). Cross-session `stop`/`resume` lifecycle controls stay host-only.
- **Host promoter roots (`A4`)**: Approved shared installation destinations come solely from trusted host config/defaults (`GOBIN` or `GOPATH/bin` for `harnez` + host-generated relative `⚙ -> harnez` symlink, and `XDG_DATA_HOME/man/man1` or `~/.local/share/man/man1` for man pages). Staged artifacts are copied and verified as immutable host-owned temp files before `O_DIRECTORY|O_NOFOLLOW`-pinned atomic per-file promotion.

## Architect Milestone Plan (M7–M17)

Assessed read-only by `1181-architect` (`codex:sol:med`) on `2026-11-16`:

- **M7 — Freeze ownership/security contract (Current milestone).**
  - Scope & files: `docs/SessionIsolation.md`, `issues/1181-isolate-harnez-session-state-and-constrain-installation-destinations.md`, new state/destination YAML spec and JSON schema under `spec/` (plus any embedded spec registration/tests required by `docs/Spec.md`).
  - Pre-work: Incorporate confirmed decisions A1–A4; enumerate the complete path/operation ownership matrix across `(a) runner-scoped ($TAIA_SESSION_DIR)`, `(b) collector/broker-owned`, `(c) intentionally shared/read-only` (explicitly covering `livefetchcache`, `agymeter` CA/key/bundle/backoff, `logging`, `feedback`, `bench`, `legacyarchive`, `subagent/agy.go` shim provisioning, and repo `.harnez` quota/checkout state); document approved promoter roots and explicit non-claims (no read secrecy when `/` is readable, no shared-checkout isolation, no arbitrary-code trust).
- **M8 — Centralize runner-local state.** Files: `internal/runstate/paths.go`, `internal/usage/{statecache,fetchdurations,killlog,passive_debug}.go` + tests. Preserve host defaults; fail closed on malformed `$TAIA_SESSION_DIR`; verify concurrent sessions, symlinks, and zero canonical-store side effects.
- **M9 — Isolate AGY and auxiliary writes.** Files: `internal/agymeter/meter.go`, `internal/usage/agy.go`, `internal/{logging,feedback,bench,subagent}` helpers + tests. Isolate AGY CA/key/bundle/log/backoff per session; keep shared shim provisioning host-only; explicitly exclude CA confidentiality under global reads.
- **M10 — Separate runner telemetry from collector administration.** Files: `internal/server/{spec,client,server}.go`, `cmd/harnez/telemetry_server.go`, telemetry callers + tests. Add inherited-FD runner transport cleanly split from host admin routes (`/shutdown`, `merge_legacy`); add bounded session spool/replay and host-only migrations (`DefaultDBPath` side effects removed for runners).
- **M11 — Route shared usage and policy access.** Files: `internal/usage/{statecache,history,livefetchcache,legacyarchive}.go`, `internal/modelblock/store.go`, `internal/subagent/tier_overrides.go` + tests. Collector RPC for canonical snapshots/history; read-only shared policy + runner overlays.
- **M12 — Add authorized registry operations.** Files: `internal/subagent/session.go`, `cmd/harnez/stats_agents.go`, new registry RPC/client files + tests. Own-session register/update/query via launch-bound channel; reject forged session IDs, paths, or PIDs.
- **M13 — Authorize lifecycle controls.** Files: subagent launch/resume/stop call sites, registry RPC handlers + tests. Keep cross-session stop/resume host-only; bind launch identity and fail closed on legacy missing-identity records.
- **M14 — Implement the host promoter.** Files: `internal/selfinstall` promotion helpers/tests, promoter spec/schema tests. Implement `O_DIRECTORY|O_NOFOLLOW`-pinned directory handling, immutable temp copy/validation, exact allowlist (`harnez`, `⚙ -> harnez`, man manifest), per-file atomic rename + cleanup.
- **M15 — Connect installation front doors.** Files: `scripts/stage-install{.sh,-test.sh}`, `Makefile`, `cmd/harnez/{install,man}.go`, `internal/selfinstall/install.go`, installation docs. Wire staging build/man generation -> host promoter, bypassing `goself` shared side effects in constrained/runner mode.
- **M16 — Establish the bwrap prerequisite/profile contract.** Files: `internal/exec/exec.go` + tests, `docs/SessionIsolation.md`, issue 1181 evidence. Canary-check `Failed to make / slave: Operation not permitted`, `uid map: Permission denied`, `/tmp` masking of `$TAIA_SESSION_DIR`, and `../taia` `--allow-bwrap` boundary; fail closed cleanly on unsupported hosts.
- **M17 — Run integrated acceptance and migration checks.** Files: Linux denial harness/tests, `scripts/stage-install-test.sh`, docs, issue 1181. Exercise concurrent normal operations + filesystem/IPC negative tests under live Taia Landlock enforcement on a supported host.

## Milestone 7 — Ownership contract

The ownership matrix, A1–A4 trust boundaries, promoter destinations and explicit
non-claims were recorded in `docs/SessionIsolation.md` and
`spec/session_isolation.yaml` with schema and contract tests (`f87b48db`).
Independent review (`1181-m7-review`, `codex:sol:low`) identified major
contract/validation gaps that are folded into M8 pre-work below. Ticket remains open.

## Milestone 8 — Pre-Work / Required Refinements & Runner-Local State

M8 landed on `sprint/1181` (`ad1cb455`) with M7 contract refinements and runner usage
paths centralized via `internal/runstate.UsageStateDir` across `statecache`,
`fetchdurations`, `killlog`, `passive_debug`, `power_peaks`, `remote_load_cache`,
`collector_store`, `collector`, and `cmd/harnez/{usage_cmd,server_cmd}.go`.
Fresh `HTO=0 make test-q1` passed (`internal/runstate`, `internal/sessioncontract`,
`internal/usage`, `cmd/harnez`, `scripts/stage-install-test.sh`). Independent
review (`1181-m8-review`, `codex:sol:low`) caught remaining live-fetch writes,
discarded errors, and test gaps folded into M9 pre-work below. M9–M17 remain open.

## Milestone 9 — Pre-Work / Required Refinements & Isolate AGY / Auxiliary Writes

M9 landed across M9a–M9d (`64403936`, `5ee6ada0`, `7e0d68c7`, `c771270d`) on
`sprint/1181` with M8 pre-work (`livefetchcache`, `power_peaks`/`power_box`,
error reporting, pipe-barrier concurrency test), AGY meter/CA/bundle/usage/backoff
isolation, host-only shim provisioning, auxiliary writers (`logging`, `feedback`,
`bench`), and host-only `legacyarchive` guards, verified by `HTO=0 make test-q1`.
Independent review (`1181-m9-review`, `codex:sol:low`) identified error/test
refinements folded into M10 pre-work below. M10–M17 remain open.

## Milestones 12 & 13 — Authorized own-session registry (`own_register`, `own_update`, `own_query`) & lifecycle guards (A3)

Added `registry_path: /agents/registry` to `spec/server.yaml` + schema, `internal/server/runner_registry.go` (`RunnerLaunchBinding` minting per-launch cryptographic `LaunchID` + `BindChildPID` process-tree validation + strict wire schema rejecting unknown/path/parent fields and forged PIDs), `BindChildPID` across all 5 launchers, `internal/subagent/{session,runner_registry}.go` (auto-stamped `LaunchID` on `Create`, fail-closed `DefaultStoreDir()` guard in runner mode, `RegisterOwnSession`/`UpdateOwnSession`/`QueryOwnSession`), `cmd/harnez/agent_registry_server.go` (host registry handler rejecting legacy missing-`LaunchID` records with 403), and runner-mode host-only lifecycle guards on `canManageTarget`, `stop`, `resume`, and `stats --agents`. Verified via `HTO=0 make test-q1` (`cmd/harnez` 61.751s, `internal/server` 0.176s, `internal/subagent` 9.102s) and `make install`. M14–M17 remain open.

## Milestone 11 — Usage reads (`usage_read`) and read-only global policy overlays (A2)

Added `usage_query_path: /usage/query` to `spec/server.yaml` + `spec/schemas/server.schema.json`, `RunnerOpUsageRead` (`usage_read`) + `ServeRunnerBrokerWith` + `QueryRunnerUsage` (`internal/server/runner_usage.go`), read-only `/usage/query` service routes (`cmd/harnez/telemetry_server.go`), and broker-routed runner reads across `internal/usage/{readstore,statecache,history,quota_history,store}.go` + `internal/usagecli/client.go` (eliminating runner-mode direct `telemetry.sqlite` access and `EnsureLocalServerFunc` probing). Implemented A2 policy isolation: `runstate.PolicyDir`, read-only global `modelblock` + runner overlay merging and unblock guards (`internal/modelblock/store.go`, `cmd/harnez/models_block.go`), and read-only global `tier_overrides` + runner overlay merging/clearing (`internal/subagent/tier_overrides.go`). Verified via `HTO=0 make test-q1` (`cmd/harnez` 61.687s, `internal/modelblock` 0.011s, `internal/runstate` 0.004s, `internal/server` 0.166s, `internal/subagent` 9.078s, `internal/usage` 7.964s, `internal/usagecli` 0.025s) and `make install`. M12–M17 remain open.

## M10b-2 — Launcher/Taia FD plumbing & runner channel hardening

Added `internal/server/runner_launch.go` (`PrepareCmdRunnerBroker`) and wired host relay / runner FD forwarding across all 5 harnez agent/subagent launch sites (`internal/exec/exec.go`, `internal/subagent/{process,codex,interactive}.go`, `cmd/harnez/subagent.go`). Verified live `../taia` behavior: Taia passes only FDs 0–2 into `bwrap`, so Taia-wrapped runners see no inherited broker FD and fail closed to their `$TAIA_SESSION_DIR` spool; direct host-launched runners and `bwrap` with `--bind <session> <session>` preserve the `SOCK_SEQPACKET` broker FD cleanly. Folded all 6 `1181-m10b1b-fix-review` hardening items (non-blocking `Sendmsg`, `MSG_CTRUNC` close/reject, 32-stream cap + idle deadline, clean EOF `wg.Wait`, permanent vs transient 4xx replay exclusion, `MaxFrameBytes > ReplayMaxBytes` spec validation, multi-worker concurrency barrier, and real `keep.json` 413 test) and documented the FD contract in `docs/SessionIsolation.md`. Verified via `HTO=0 make test-q1` (`cmd/harnez` 61.149s, `internal/exec` 4.423s, `internal/server` 0.093s, `internal/subagent` 9.210s) and `make install`. M11–M17 remain open.

## M10b-1b — Inherited-FD runner channel, relay, and bounded replay

Added `runner_channel` settings and schema constraints in `spec/server.yaml` and `spec/schemas/server.schema.json`, fail-closed `Config.Socket()` guard when `TAIA_SESSION_DIR` is set, `internal/server/runner_channel.go` (`AF_UNIX` `SOCK_SEQPACKET` broker FD with per-request `SCM_RIGHTS` stream handoff, length-prefixed JSON framing, strict field/route/kind allowlisting rejecting `/shutdown`, `merge_legacy`, `ensure_schema`, and oversize frames/batches), `RelayRunnerBroker`, typed `HTTPStatusError`, per-record replay outcomes, bounded runner spool/replay (`SendRunnerTelemetry` / `ReplayRunnerSpool` without sibling scanning), and folded M10b-1a/M10b-1b review fixes (concurrent subprocesses, timeout follow-up isolation, wire byte limits, mixed quarantine batch tests). Verified via `HTO=0 make test-q1` (`internal/server` 0.073s, `cmd/harnez` 61.148s) and `make install`. M10b-2 (launcher/taia FD plumbing + canary) and M11–M17 remain open.

## M10b-1a — Telemetry receipts and runner spool path

Runner telemetry defaults fail closed, `server.Config.TelemetrySpoolDir()` routes to `$TAIA_SESSION_DIR/state/telemetry-spool` via `runstate.TelemetrySpoolDir`, and collector records carry stable IDs applying `telemetry_receipts(record_id)` + telemetry writes in one SQLite transaction (pre-existing ID-less records retain legacy delivery semantics). Verified via `HTO=0 make test-q1` (exit 0) and `make install`. M10b-1b remains open for the inherited-FD runner channel and sibling-blind replay integration.

## Milestone 10 — Pre-Work / Required Refinements & Separate Runner Telemetry

### M10a — M9 review refinements

M10a (`bc1b5066`) addresses AGY meter path and backoff write error reporting, removes the feedback home fallback, and adds producer/reader, shared-write, cache round-trip, and invalid-root regressions. A host Quota-1 run failed because two AGY test fixtures lacked mode-0700 runner roots; the fixtures now set that mode explicitly, and `HTO=0 make test-q1` passed. M10b telemetry and broker work remains open.

### M10 Implementation Scope (Owner Decision A1)
- **Runner vs Admin Transport (`internal/server/{spec,client,server}.go`, `cmd/harnez/telemetry_server.go`)**: split collector HTTP/RPC handling cleanly between the host admin server and a runner channel authenticated via a **launch-bound inherited connected socketpair/FD** (A1) passed by the trusted host launcher. Reject or never expose host-only admin routes (`/shutdown`, `merge_legacy`, caller-supplied DB/source paths) on the runner channel; keep host admin routes functional for host callers. Do not rely on `0600` socket permissions or readable tokens for session auth.
- **Runner Telemetry Spool & Replay (`internal/server/{spec,client}.go`, `internal/runstate/paths.go`)**: route runner spool paths under `$TAIA_SESSION_DIR` via `internal/runstate`; support bounded spool records and duplicate-safe replay without scanning sibling session directories.
- **Host-Only Migrations (`internal/telemetry/telemetry.go:30`, callers)**: ensure runners never trigger `DefaultDBPath` migration/removal writes or shared writable SQLite side effects.
- **M10 Tests**: verify inherited-FD runner auth (missing/invalid FD fails closed), forbidden admin routes (`/shutdown`, `merge_legacy`, arbitrary paths) rejected on runner channel while passing on host admin channel, oversized payload rejection, duplicate replay deduplication, collector outage -> runner spool -> bounded replay, invalid `$TAIA_SESSION_DIR` rejection, and untouched shared SQLite/migrations in runner mode.

### Pre-Work / Required Refinements (from M8 review of `ad1cb455`)
- **Isolate `livefetchcache` (`internal/usage/livefetchcache.go:60`)**: route default live-fetch cache paths through `internal/runstate.UsageStateDir` under `$TAIA_SESSION_DIR` when set, and skip shared legacy file migration writes in runner mode so runner collection never touches shared XDG cache files.
- **Stop silently discarding runner path/broker errors**: inspect `recordFetchDuration`, `capturePassiveNoQuota`, `powerPeaksPath` (must not return `""` into store paths on error), `ReadCachedRemoteLoadEntry`, `CollectRegisteredCatchup`, and `CollectRegistered` (`publishAgentObservations` error). Propagate or fail closed visibly without writing to shared/relative fallback paths.
- **Strengthen M8 regression tests (`internal/usage/runner_state_test.go`)**: cover `livefetchcache` isolation and zero legacy migration in runner mode, `power_peaks` and `remote_load_cache` isolation, explicit directory overrides, invalid runner roots through actual writers, assertions that shared dirs stay empty *after* collector guard calls, and an explicit synchronization barrier across concurrent child writers in `TestRunnerProcessesWriteSeparately`.

### M9 Implementation Scope
- **AGY isolation (`internal/agymeter/meter.go`, `internal/usage/agy.go`)**: move AGY meter CA/key, certificate bundle, `usage.jsonl`, and auth-backoff marker (`usage/agy.go:149`) together into runner storage under `$TAIA_SESSION_DIR` (`$TAIA_SESSION_DIR/state/agymeter` or `runstate` helper) so producer and reader agree; support legacy host defaults when `TAIA_SESSION_DIR` is unset, prevent shared writes in runner mode, and keep no CA-confidentiality claim under global reads.
- **Shared runtime shims (`internal/subagent/agy.go`)**: keep shared runtime shim provisioning host-only; launching inside a runner (`TAIA_SESSION_DIR` set) must not rewrite shared shim files.
- **Auxiliary writes (`internal/logging/logging.go`, `internal/feedback/feedback.go`, `internal/bench/store.go`, `internal/usage/legacyarchive.go`)**: isolate runner-local writes under `$TAIA_SESSION_DIR` via `internal/runstate` helpers while keeping host defaults and legacy migration/export host-only.
- **M9 Tests**: producer/reader CA+bundle+`usage.jsonl` agreement, backoff isolation, host-only shim guard, auxiliary writers under `$TAIA_SESSION_DIR`, malformed runner root fail-closed checks, and zero shared writes.

### Pre-Work / Required Refinements (from M7 review of `f87b48db`)
- **Resolve `livefetchcache` ownership conflict**: reconcile `docs/SessionIsolation.md` (currently assigns live-fetch cache to `runner`) with `spec/session_isolation.yaml` (currently assigns `canonical_usage_snapshots_history_livefetchcache` to `broker`); freeze one unambiguous owner (or explicitly separate runner-local cache under `$TAIA_SESSION_DIR` vs host-only legacy/canonical handling) in both doc and spec.
- **Eliminate `docs/Spec.md` duplication & tighten validation**: make `spec/schemas/session_isolation.schema.json` the single source of truth (add `minItems`, enforce uniqueness and required host-only/ownership constraints via schema rather than Go literals); have `internal/sessioncontract.Load()` decode strictly (`KnownFields`) and validate against the embedded JSON schema, limiting Go `Validate()` to cross-field invariants (`unique(Ownership.Resource)`, `disjoint(RunnerOperations, HostOnlyOperations)`, `AliasTarget == Binary`).
- **Contract API hygiene & tests**: add doc comments to exported `Ownership`, `Contract`, `Load`, `Validate`; remove premature `ValidateRoot` helper (belongs to M14 `internal/selfinstall`); test duplicate ownership resources, duplicate/conflicting operations, missing/empty operation lists, and unknown fields.

### M8 Implementation Scope
- Centralize remaining runner-local paths under `$TAIA_SESSION_DIR` via `internal/runstate/paths.go` for `internal/usage/{statecache,fetchdurations,killlog,passive_debug}.go` (preserving legacy host defaults when `TAIA_SESSION_DIR` is unset, honoring explicit dir overrides, and failing closed on malformed/symlinked runner roots — note `statecache.go:380` canonical snapshot store coupling must not silently write shared canonical state in runner mode).
- Add regression tests across all 4 usage writers + `internal/runstate` covering explicit overrides, malformed/symlink roots, `0700`/`0600` private permissions, concurrent sessions, and absence of shared canonical writes.
