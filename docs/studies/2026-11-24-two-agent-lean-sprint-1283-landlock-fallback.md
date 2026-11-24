---
kind: sprint
date: 2026-11-24
ticket: 1283
branch: sprint/1283
models:
  architect: agy:argon:med
  developer: agy:hydrogen:med
  reviewer: agy:neon:med # switched from claude:opus:high after Claude 5h limit
---

# Two-Agent Lean Sprint: #1283 — Landlock Fallback in `harnez exec --quota-1`

## 1. Setup & Filing

- Branch: `sprint/1283` (team repo, protected `main`).
- Ticket filed: [1283-fallback-to-landlock-in-harnez-exec-quota-1-when-nested-bwrap-fails.md](../../issues/1283-fallback-to-landlock-in-harnez-exec-quota-1-when-nested-bwrap-fails.md) (`357ef7e0`).
- Origin: Coordination with peer `taia` confirmed Taia needs no changes; `harnez exec --quota-1` should stack its own Linux Landlock layer rather than relying on nested `bwrap` or spoofable `TAIA_*` env vars.
- Searcher step: **skipped** — `argon:med` performed live-code architect refinement with exact `file:line` citations across `internal/exec/`, `internal/server/`, `cmd/harnez/`, `go.mod`, and `spec/session_isolation.yaml`.

## 2. Architect Refinement (`argon:med`)

- Session: `argon-1283` (`f30ba0f9-80b7-43e3-acb9-245bb35a6527`), 7m20s, 1,870,490 tokens (incl. cached).
- Key architectural findings recorded in #1283:
  1. `golang.org/x/sys v0.47.0` is already in `go.mod` — zero new dependencies needed.
  2. Self-reexec helper (`MaybeRunSandboxHelper` with `__landlock-exec --`) using `runtime.LockOSThread()` + `PR_SET_NO_NEW_PRIVS` + `SYS_LANDLOCK_RESTRICT_SELF` + in-place `unix.Exec` preserves PID/PGID, signals, timeout killer, and avoids restricting the parent `harnez` process.
  3. Must clear `FD_CLOEXEC` on validated `HARNEZ_BROKER_FD` before `unix.Exec` because `internal/server.init()` sets `FD_CLOEXEC` on startup.
  4. Must include `LANDLOCK_ACCESS_FS_REFER` (ABI v2+) for cross-subdir renames, `LANDLOCK_ACCESS_FS_TRUNCATE` (ABI v3+), and safe device node write rules (`/dev/null`, `/dev/ptmx`, `/dev/pts`, `/dev/shm`, etc.).
  5. Short `hz-q1-*` private temp dir for `TMPDIR` and `XDG_RUNTIME_DIR` keeps Unix socket paths under 108 bytes (#1259).
- Split into 2 milestones:
  - **M1**: Linux Landlock confinement helper & platform build split (`internal/exec/landlock_linux.go`, `landlock_other.go`, `landlock_test.go`, `MaybeRunSandboxHelper` hooks in `cmd/harnez/main.go`, `internal/exec/gittest_main_test.go`, `cmd/harnez/main_env_test.go`).
  - **M2**: `runExecWrapper` fallback wiring, private `TMPDIR`/`XDG_RUNTIME_DIR`, acceptance tests, and `docs/SessionIsolation.md`.

## 3. Milestone 1 (`M1`: Linux Landlock Confinement Helper & Platform Build Split)

- Developer: `hydrogen:med` (`dev-1283`, 5m57s, 673,747 tokens incl. cached), commit `b845e492`.
- Host verification: all 5 kernel Landlock tests ran unskipped and passed (`0.099s`); `dev-1283` ran `go test -race -count=1 ./internal/exec/... ./cmd/harnez/...` (green).
- Reviewer: `opus:high` hit Claude 5h usage limit; user approved `neon:med` (`rev-1283-m1`, 1m40s, 200,864 tokens incl. cached).
  - Core Landlock syscall/thread/FD handling and trigger contract `[OK]`.
  - Caught `[MED]` test gap: `os.Pipe()` did not trigger `internal/server.init()` `FD_CLOEXEC` (needs `SOCK_SEQPACKET` `unix.Socketpair`), plus `[LOW]` ABI v3 truncate gate and `F_SETFD` error check — embedded as M2 Pre-Work in #1283.

## 4. Milestone 2 (`M2`: `runExecWrapper` Fallback Wiring, Private `TMPDIR`, & Acceptance Tests)

- Developer: resuming `dev-1283` (`hydrogen:med`) with M2 Pre-Work + M2 implementation.
