# 995 — Statusline fails with Text file busy while the harnez binary is being replaced

**Status**: Backlog — rare: owner sees it seldom (2026-10-28); fix when touching install
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 989 (stale collector after install); goself 006 (restart detection after install)

---

/goal Replacing `~/go/bin/harnez` (via `make install`, `go install`, or a harnez/goself self-install path) never
makes a concurrent `harnez` exec fail with `Text file busy`; the new binary appears by atomic rename. Stop and
report if the writer turns out to be `go install` itself and the only fix is outside this repo.

## 1. Problem & Motivation

On 2026-10-27 the agy (Antigravity CLI) status line showed:

```
⚠ Statusline error:
command failed: exit status 126 (stderr: sh: line 1: /home/uwe/go/bin/harnez: Text file busy)
Full logs at: /home/uwe/.gemini/antigravity-cli/log/cli-20261027_112542.log
```

Exit 126 with `ETXTBSY` means the shell tried to exec the file while another process had it open for writing.
Status lines run harnez every few seconds in every open session, so any in-place write of the binary races them.
The error is transient, but it is shown to the user and the same race can hit hooks (telemetry senders) and
`harnez agent` workers started during an install.

## 2. Findings / Unknowns

- `make install` runs `go install` (Makefile `install` target). `go install` normally renames from the build
  cache, which is atomic on the same filesystem; it copies in place when the cache is on another filesystem.
  Not yet checked which writer was active at 11:25 (make install, release install, goself, or a copy).
- Atomic replace: write `harnez.tmp.<pid>` in the same directory, `chmod`, `rename` over the target. Running
  processes keep the old inode.

## 3. Plan & Verification

1. Identify the writer (shell history / session logs around 2026-10-27 11:25, one targeted look) and every
   in-repo code path that writes the installed binary.
2. Make each such path write-temp-then-rename; if `go install` is the writer, have `make install` build to
   `bin/` and install by rename.
3. Verify: loop `harnez --version` in the background while running `make install` several times; zero
   exit-126 failures.
