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

## Initial bounded milestone: current contracts

These are source-level contracts, not a host inspection or proof of isolation. Paths depend on the process environment and OS; no particular sandbox or runtime directory is assumed.

- Session resolution uses explicit/environment IDs first; its fallback reads, creates and updates PPID-derived lock files under `$HOME/.harnez/sessions` (directory requested as 0755, lock files 0644, subject to umask). Ticket history and session state also use this directory; session state writes JSON with requested mode 0644. These modes do not isolate processes sharing a UID, and existing paths retain their existing permissions.
- Agent registry defaults to `$HOME/.harnez/agents`, configurable with `--store-dir`. It creates the directory with requested mode 0700, serializes registry operations with `.registry.lock` and flock, and writes session JSON via a temporary file and rename. Async stdout/stderr logs also live in the store. Locks and atomic rename coordinate cooperating writers; they do not authenticate writers or enforce cross-session authorization.
- Exec process records are `$XDG_RUNTIME_DIR/harnez/procs/<pgid>.json` when that variable is set, otherwise `$HOME/.harnez/run/procs/<pgid>.json`. Creation requests 0700 for directories and 0600 for files; existing paths are not secured by those requests. Writers create/overwrite records and cleanup reads and can remove them. Linux cleanup relies on readable `/proc` process identity and UID data, and kernel signal permissions; other hosts or restricted `/proc` may not supply these assumptions.
- `make install` builds `./harnez`, runs `go install` (destination follows Go's `GOBIN`/`GOPATH` rules), generates the command tree's man pages and creates or replaces the `⚙` symlink at `$(go env GOPATH)/bin/⚙`; it falls back to `$HOME/go/bin/⚙` if that command fails. The Makefile uses the full, unsplit `go env GOPATH` output as one path. Man pages go to `/usr/local/share/man/man1` as root, otherwise `$XDG_DATA_HOME/man/man1` or `$HOME/.local/share/man/man1`, with a system fallback. `harnez man --dir` selects another directory. This is not an allowlisted installer; destination directories and path components must be trusted against symlink and rename substitution. A workspace build or custom destination alone does not prove safe installation into shared paths.

## Process safety characterization and options for review

- Cleanup defaults to reporting; `--kill` enables signals. For visible group members it requires known matching UID, checks the leader start time when present, and re-reads group members before each signal. It treats a matching same-UID owner as alive (zero owner start time matches any start time). A stopped group or dead owner is eligible; a zero recorded leader start time is not signalled. A group with no visible members is retained as unknown if the kernel says it exists. Checks and signals are separate operations, and a missing leader is not authenticated by a leader check.
- `TestCleanProcsForgedRecordCanSignal` supplies a forged record with the correct fake leader start time and a dead fake owner. Cleanup issues fake SIGTERM. This passing characterization demonstrates the absence of record authentication, not protection against forgery; no real process is signalled.
- Agent stop builds process, provider and sometimes worker roots from the stored session. `TreeMembers` checks a nonzero root start time against a visible PID, tracks same-UID descendants and eligible group members, and `StopTree` signals individual PIDs with TERM then KILL. A zero root start time skips the reuse comparison; the fake-reader/fake-signal test demonstrates that a same-UID reused PID can receive fake TERM. A distinct worker root is included only with a nonzero start time. PID checks and signals are not atomic, and visibility and UID checks are not authorization of a writable session record.
- Owner choices: reject unknown start times for signalling and report an unverifiable stop (assess legacy and provider-group compatibility); make cleanup report-only for records from an untrusted registry; or move record creation and signalling behind a separately trusted authority with session-scoped authorization. A start-time check alone cannot authenticate a forged record or eliminate signal races. For installation, consider a trusted installer that validates fixed approved destination names and directory handles, rejects symlink/path substitution, and installs a workspace-built artifact; define who approves and owns those directories before design. No option is implemented or selected here.
