# 553 — Tests delete real ~/.local/bin/harnez-agy and ~/.harnez/shims via CleanAll without HOME isolation

**Status**: Closed — internal/claude TestMain sets temp HOME; make test-q1 green, real launcher and shim untouched
**Priority**: P1
**Severity**: High
**Category**: Bug / Tests
**Related**: [[550-harnez-apply-installs-harnez-agy-launcher-in-local-bin-to-shim-agy]]

---

## Problem

`CleanAll` (internal/claude/apply.go ~1720) removes the harnez-agy launcher and the bash shim under
`os.UserHomeDir()`. Tests calling `CleanAll` without `t.Setenv("HOME", t.TempDir())` (agents_profile,
docs, toolfeedback, telemetry_hook, gear, integration) delete the user's real files on every test run.
Seen 2026-09-24: after sprint 551's `make test-q1`, both files were gone; a running harnez-agy session
fell back to the hook rewrite ("harnez exec" shown in the agy UI again). Restored with `harnez apply`.

## /goal

No test in the repo can touch the real home: `internal/claude` (and `cmd/harnez`) set HOME to a temp
dir for every test (e.g. TestMain), and a guard test fails if a test run changes real-home files.
