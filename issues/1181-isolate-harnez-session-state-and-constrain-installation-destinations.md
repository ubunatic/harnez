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

## Milestone 3 — Pre-Work / Required Refinements

Independent review of 052b4e65 found that an invalid runner state path can return from the read hook before emitting an already-computed denial (cmd/harnez/hook.go), potentially allowing a prohibited read. Preserve the denial response even when state writes are unavailable, and add a regression for a prohibited read with an invalid runner root. Quota-1 remains unverified: with taia --allow-bwrap, bwrap failed to make / slave with Operation not permitted before tests started.

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
