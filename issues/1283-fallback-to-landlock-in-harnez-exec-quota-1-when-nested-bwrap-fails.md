# 1283 — Fallback to Landlock in harnez exec --quota-1 when nested bwrap fails

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: #1181, #1259

---

## 1. Problem & Motivation

Inside sandboxed sessions such as Taia (without `--allow-bwrap`), `harnez exec --quota-1` (used by `make test-q1`) fails with:

```text
bwrap: setting up uid map: Permission denied
```

`harnez exec --quota-1` currently wraps the child process in `bwrap` (`internal/exec/exec.go`) and fails closed when nested user namespaces / UID mapping are denied.

We must **not** skip confinement based on environment variables (e.g., `TAIA_SANDBOX_MODE`, `TAIA_ALLOW_BWRAP`), because env vars are informational and spoofable by any parent process.

Instead, per coordination with `taia`, `harnez exec --quota-1` on Linux should support **Landlock** (`PR_SET_NO_NEW_PRIVS` + `landlock_create_ruleset` / `landlock_restrict_self`, ABI v1+) so it can stack its own confinement layer without needing user namespaces or privileges. Landlock domains stack monotonically (a nested layer can only narrow access, never widen it), so Taia's confinement remains intact and Harnez's confinement guarantee holds regardless of parent.

---

## 2. Technical Specification & Architect Refinement (`argon:med`)

### Live-Code Citations

- **`bwrap` Sandbox Construction & Cache/Session Collection**:
  - `internal/exec/exec.go:273-297`: `quota1SandboxArgs` / `quota1SandboxArgsWithSession` constructs `bwrap` argv (`--ro-bind / / --dev /dev --proc /proc --tmpfs /tmp`, binds `cwd`, `goCache`, `goModCache`, masks `runtimeDir` with `--tmpfs`, re-binds validated `sessionDir` after tmpfs masks, and sets `--chdir <cwd> -- <args...>`).
  - `internal/exec/exec.go:299-335`: `quota1SandboxCommand(args, cwd)` loads `sessioncontract.Load()`, validates `$TAIA_SESSION_DIR` via `validateBwrapSessionBind`, checks `runtime.GOOS != "linux"`, looks up `bwrap`, queries `go env GOCACHE GOMODCACHE`, and reads `XDG_RUNTIME_DIR`.
- **`bwrap` Error Detection, Probe & Session Dir Validation**:
  - `internal/exec/bwrap_contract.go:17-59`: `hasBwrapDiagLine` and `DiagnoseBwrapError` match `bwrap:` lines and substrings (`"setting up uid map"`, `"failed to make / slave"`, `"create a new namespace"`, `"creating new namespace failed"`).
  - `internal/exec/bwrap_contract.go:64-90`: `ProbeBwrapSupport(ctx)` runs `bwrap --ro-bind / / --dev /dev --proc /proc -- true`.
  - `internal/exec/bwrap_contract.go:92-114`: `validateBwrapSessionBind(sessDir)` enforces clean absolute path, non-symlink real dir, mode `0700`, owned by `os.Geteuid()`, `filepath.EvalSymlinks(sessDir) == sessDir`.
- **Child Exec Flow, Env, Signals & Broker FD**:
  - `internal/exec/exec.go:696-720`, `747-818`, `854-858`: `runExecWrapper` sets `Setpgid: true`, `HARNEZ_SANDBOX=quota1`, `HARNEZ_QUOTA_1_ACTIVE=1`, prepares `HARNEZ_BROKER_FD` (`ExtraFiles` FD 3), binds PID, manages timeout/stopped-group monitors, and runs `DiagnoseBwrapError` on non-zero exit.
  - `internal/server/runner_channel.go:26-33`: `init()` marks any inherited `HARNEZ_BROKER_FD` `FD_CLOEXEC` at startup.
- **Entrypoints & Dependencies**:
  - `cmd/harnez/main.go:126-138`, `internal/exec/gittest_main_test.go`, `cmd/harnez/main_env_test.go`.
  - `go.mod:14`: `golang.org/x/sys v0.47.0` is already a direct dependency with all `unix.SYS_LANDLOCK_*` and `unix.LANDLOCK_*` symbols. **Zero new dependencies.**
  - Platform build tag pattern: `internal/selfinstall/promote_linux.go` (`//go:build linux`) and `promote_other.go` (`//go:build !linux`).

### Key Architecture Decisions

1. **No Double-Running User Command**:
   - Probe `bwrap` first (`bwrap --ro-bind / / --dev /dev --proc /proc -- true`, bounded timeout) before spawning the user command.
   - Always validate `$TAIA_SESSION_DIR` via `validateBwrapSessionBind` first; fail closed on malformed `$TAIA_SESSION_DIR` before either backend runs.
   - Never consult `TAIA_SANDBOX_MODE`, `TAIA_ALLOW_BWRAP`, or any env var to skip confinement.
   - If `bwrap` probe succeeds -> use `bwrap`.
   - If `bwrap` is unavailable or fails with namespace / UID-map / mount-propagation permission error (`EPERM`/`EACCES`, `"setting up uid map"`, `"failed to make / slave"`, `"create a new namespace"`, `"creating new namespace failed"`) and `probeLandlockABI()` returns `abi >= 1` -> fall back to Landlock.
   - If both `bwrap` and Landlock fail/are unavailable -> fail closed with actionable error without executing the user command.

2. **Safe Landlock Application via Self-Reexec Helper (`MaybeRunSandboxHelper`)**:
   - Never call `landlock_restrict_self` in the parent `harnez` process (it would permanently restrict the parent).
   - Instead, `runExecWrapper` launches `os.Executable()` with `["__landlock-exec", "--", args...]` and JSON-encoded write dirs in `HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS`.
   - `internalexec.MaybeRunSandboxHelper()` is called at the very top of `cmd/harnez/main.go` `main()`, `internal/exec/gittest_main_test.go` `TestMain`, and `cmd/harnez/main_env_test.go` `TestMain`.
   - In the helper:
     1. Check `len(os.Args) >= 4 && os.Args[1] == "__landlock-exec" && os.Args[2] == "--"` and non-empty `HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS`.
     2. Call `runtime.LockOSThread()` **before** `PR_SET_NO_NEW_PRIVS`, `SYS_LANDLOCK_RESTRICT_SELF`, and `unix.Exec` (critical on Linux < 6.10 so the goroutine cannot migrate off the restricted OS thread before `execve`).
     3. Parse JSON write dirs, unset `HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS`.
     4. Apply Landlock ruleset (exit 126 with clear stderr diagnostic on failure).
     5. Clear `FD_CLOEXEC` on validated `HARNEZ_BROKER_FD` (`fd >= 3`) before `unix.Exec` (counteracting `internal/server.init()`).
     6. Resolve `os.Args[3]` via `exec.LookPath` and call `unix.Exec(targetPath, os.Args[3:], os.Environ())` in-place so PID, PGID (`Setpgid: true`), signal/timeout killing, broker PID binding, and stdin/stdout/stderr remain identical to `bwrap`.

3. **Landlock ABI v1+ Ruleset & Path Grants**:
   - ABI check: `unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)`; require `abi >= 1`.
   - `handledAccessFS`: ABI v1 read/exec (`EXECUTE | READ_FILE | READ_DIR`) + write/mutate (`WRITE_FILE | REMOVE_DIR | REMOVE_FILE | MAKE_CHAR | MAKE_DIR | MAKE_REG | MAKE_SOCK | MAKE_FIFO | MAKE_BLOCK | MAKE_SYM`).
   - If `abi >= 2`: include `LANDLOCK_ACCESS_FS_REFER` (required for cross-subdir `rename`/`link` inside writable trees on Linux 5.19+).
   - If `abi >= 3`: include `LANDLOCK_ACCESS_FS_TRUNCATE`.
   - Omit `LANDLOCK_ACCESS_FS_IOCTL_DEV` so TTY/PTY `ioctl` calls work.
   - Rules:
     - `/` -> `EXECUTE | READ_FILE | READ_DIR`.
     - Existing paths in `[cwd, goCache, goModCache, sessionDir, tmpDir, runDir]` plus `/dev/shm` and `/dev/pts` (if present) -> full `handledAccessFS`.
     - Existing safe device nodes (`/dev/null`, `/dev/zero`, `/dev/full`, `/dev/random`, `/dev/urandom`, `/dev/tty`, `/dev/ptmx`) -> `READ_FILE | WRITE_FILE` (plus `TRUNCATE` on ABI v3+).

4. **Private `TMPDIR` & `XDG_RUNTIME_DIR` (Short Paths for #1259)**:
   - Create a private `0700` `sandboxRoot` with short prefix `hz-q1-*` (prefer `os.MkdirTemp("", "hz-q1-*")`, falling back to `sessionDir` or `cwd` if `os.TempDir()` is read-only) so Unix domain socket paths stay under 108 bytes.
   - Create `tmp` (`0700`) inside `sandboxRoot` -> set `TMPDIR=tmpDir` and add to Landlock write grants.
   - If `XDG_RUNTIME_DIR` is non-empty, create `run` (`0700`) inside `sandboxRoot` -> set `XDG_RUNTIME_DIR=runDir` and add to Landlock write grants.
   - Clean up `sandboxRoot` via `os.RemoveAll` after `c.Wait()` in `runExecWrapper`.

---

## 3. Implementation & Verification Plan

### Milestone 1 (`M1`): Linux Landlock Confinement Helper & Platform Build Split

- **Write Set**:
  - Create `internal/exec/landlock_linux.go` (`//go:build linux`)
  - Create `internal/exec/landlock_other.go` (`//go:build !linux`)
  - Create `internal/exec/landlock_test.go`
  - Edit `cmd/harnez/main.go` (call `internalexec.MaybeRunSandboxHelper()` at the very top of `main()`)
  - Edit `internal/exec/gittest_main_test.go` and `cmd/harnez/main_env_test.go` (call `MaybeRunSandboxHelper()` at the top of `TestMain`)
- **Verification (`M1`)**:
  - Kernel Landlock subprocess tests in `internal/exec/landlock_test.go` (skipping only if `probeLandlockABI()` reports kernel lacks Landlock):
    - Writes inside granted `cwd` and `TMPDIR` succeed.
    - Writes to `/dev/null` succeed.
    - Cross-subdir `os.Rename` inside `cwd` succeeds (ABI v2+ `REFER`).
    - Writes, truncates, deletes, and renames in `$HOME` or a sibling temp dir outside the grant list fail with `EACCES` (`permission denied`).
    - Broker FD (`HARNEZ_BROKER_FD`) survives across `MaybeRunSandboxHelper` `unix.Exec`.
  - `GOWORK=off go test -race -count=1 ./internal/exec/... ./cmd/harnez/...` passes, and non-Linux cross-compile (`GOOS=darwin go build ./internal/exec/...`) succeeds.

### Milestone 2 (`M2`): `runExecWrapper` Fallback Wiring, Private `TMPDIR`, & Acceptance Tests

- **Write Set**:
  - Edit `internal/exec/bwrap_contract.go`
  - Edit `internal/exec/exec.go`
  - Edit `internal/exec/bwrap_contract_test.go`
  - Edit `internal/exec/exec_test.go`
  - Edit `cmd/harnez/session_isolation_acceptance_test.go`
  - Edit `docs/SessionIsolation.md`
- **Edge Cases & Negative Tests (`M2`)**:
  1. Forced Landlock fallback (fake `bwrap` printing `bwrap: setting up uid map: Permission denied` and exiting 1) -> `runExecWrapper` with `--quota-1` falls back to Landlock, grants `cwd`/`TMPDIR`/`$TAIA_SESSION_DIR`, denies `$HOME` and `/tmp` outside `TMPDIR` with `EACCES`.
  2. No double execution of user command (counter file verifies command runs exactly once).
  3. Fail-closed when both `bwrap` and Landlock are unavailable (via injectable test seam) -> exit 1 with actionable error, command never runs (`TestQuota1SandboxFailsClosedWithoutBwrap`).
  4. Malformed `$TAIA_SESSION_DIR` (mode `0755`, relative, or symlink) fails closed before fallback.
  5. Env-var spoofing (`TAIA_SANDBOX_MODE=landlock`, `TAIA_ALLOW_BWRAP=0`) ignored; writes outside grants still blocked.
  6. Private `TMPDIR` cleaned up after `runExecWrapper` returns; broker FD telemetry survives Landlock confinement.
  7. `cmd/harnez/session_isolation_acceptance_test.go`: assert `HARNEZ_QUOTA_1=1 harnez exec` works inside Taia Landlock mode (without `--allow-bwrap`).
- **Verification (`M2`)**:
  - `make install`, full `HTO=0 make test`, and live check of `HARNEZ_QUOTA_1=1 ~/.local/bin/harnez exec -- ...` (including `HTO=0 make test-q1` if applicable).

---

## 4. Decision Log

- **JSON encoding for `HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS`**: Use JSON array encoding rather than NUL-delimited env var (POSIX env values cannot contain NUL bytes; JSON handles arbitrary paths cleanly).
- **Zero new dependencies**: Use existing `golang.org/x/sys/unix` (`go.mod` v0.47.0) per `docs/Go.md`.
