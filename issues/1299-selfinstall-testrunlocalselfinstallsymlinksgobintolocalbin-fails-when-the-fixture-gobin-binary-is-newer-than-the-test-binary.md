# 1299 — selfinstall: TestRunLocalSelfInstallSymlinksGoBinToLocalBin fails when the fixture gobin binary is newer than the test binary

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: #1298 (seen in its q1 run), a840be93 (earlier TMPDIR fix to the same test), #1291 (test speed and tiers)

---

## Goal

/goal Make `TestRunLocalSelfInstallSymlinksGoBinToLocalBin` (`internal/selfinstall/install_test.go:183`) deterministic. The test must not depend on whether the fixture gobin file is newer than the running test binary. Prove it by running the test after a fresh `make install` and with a cached test binary. Stop and report if the shadow-warning logic itself is wrong rather than the test.

## Problem

`make test-q1` on `sprint/1298` (2026-11-25, right after `make install`) failed:

```
install_test.go:219: unexpected shadow warning after symlinking gobin:
  warning: .../go/bin/harnez is newer than running .../selfinstall.test; self copied the older executable
```

The test writes a fresh `stale` script into a temp gobin, then runs the self strategy, which compares the gobin file's mtime with the running executable. A test binary served from the build cache is older than a fixture written during the run, so the warning fires. The branch did not touch `internal/selfinstall`.

## Ask

Pin the fixture's mtime to before the running executable (`os.Chtimes`), or inject the clock or executable path into `Options`. Keep the `warning:` assertion.

## Decision

- Searcher skipped: the host filed this ticket in sprint 1298 from the failing log, and the developer's plan turn confirmed it against live code.
- The fix is test-only: `os.Chtimes` pins the fixture at 2000-01-01 UTC. The mtime comparison lives in the pinned `ubunatic.com/goself v0.5.0` (`shadowWarning`, `goself.go:484-502`, with `os.Executable` at `goself.go:186`). Neither Options struct has an executable field, so injecting one would mean changing the external module. The production logic is correct.
- No separate red run. The failure is already on record in the 1298 q1 log. The new test `TestRunLocalSelfWarnsWhenGoBinIsNewer` (fixture at now+1h) is the counter-check that the warning still fires. A pre-fix run would not fail reliably, because it depends on how old the cached binary is. So there is one quota run, after the fix.
