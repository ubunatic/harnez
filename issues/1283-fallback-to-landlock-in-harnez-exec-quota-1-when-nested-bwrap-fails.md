# 1283 — Fallback to Landlock in harnez exec --quota-1 when nested bwrap fails

**Status**: Open
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

## 2. Technical Specification / Findings

Initial design constraints from `taia` coordination (to be refined against live code by architect):

- **Fallback order (Linux)**:
  - Try `bwrap` first (or probe whether `bwrap` userns/UID map succeeds); if `bwrap` fails due to userns / UID map permission errors (`EPERM`/`EACCES`), fall back to Landlock-only confinement.
  - Check Landlock ABI version (ABI v1+ covers filesystem writes); **fail closed** if both `bwrap` and Landlock are unavailable.
- **Landlock write grants**:
  - Grant writes only to `cwd`, configured caches (same cache dirs `bwrap` binds), validated `$TAIA_SESSION_DIR` (if present and valid per `internal/exec/bwrap_contract.go`), and a fresh private temp directory.
  - Because Landlock does not provide a private `/tmp` tmpfs or `--dev`/`--proc` isolation, create a fresh temp dir under a granted location (or clean it up on exit), set `TMPDIR` to it, and include it in the Landlock write rules.
- **No env-var bypass**:
  - Never disable confinement based on `TAIA_*` env vars.
- **Implementation**:
  - Prefer zero new external dependencies if raw Linux syscalls (`golang.org/x/sys/unix` or `syscall`) are clean and concise per `docs/Go.md`, or evaluate existing `go.mod` deps.
  - Keep non-Linux builds compiling cleanly (`_linux.go` / `_other.go` build tags).

## 3. Implementation & Verification Plan

*Pending architect refinement (`argon:med`) into concrete milestones (`M1`, `M2`, ...), exact file:line citations, boundary constraints, edge cases, and verification criteria.*
