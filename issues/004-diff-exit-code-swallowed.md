# diff exit code 2 (error) silently swallowed

**Status:** Closed — fixed in `internal/markdown/markdown.go` and `internal/claude/apply.go` (2026-08-19)
**Severity:** Medium — false success reporting when diff fails, swallowed execution errors

## Problem

Both `internal/markdown/markdown.go` (`diffSection`) and `internal/claude/apply.go` (`diffSettingsJSON`) invoke external `diff(1)` via `cmd.Run()` and ignore the returned error:

```go
cmd := exec.Command("diff", "-u", "--label", label, "--label", label, oldFile, newFile)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
cmd.Run()        // error ignored
return true, nil
```

Standard POSIX/GNU `diff` exit status:
- **0**: Inputs are identical (note: in our Go code, equality is already pre-checked via `bytes.Equal` or string equality before `exec.Command`, returning `false, nil`).
- **1**: Files differ (normal diff output stream, expected when diffing).
- **2** (or other non-zero / exec error): Fatal error (missing `diff` binary in `$PATH`, permission denied, invalid arguments, out of file descriptors, signal termination, etc.).

Ignoring `cmd.Run()` means when `diff` binary fails to launch or exits with status 2:
1. `diffSection` / `diffSettingsJSON` returns `true, nil` — claiming changes were detected and diff output was displayed when in reality nothing (or only an error message on `stderr`) was printed.
2. `DiffAll` in `apply.go` thinks changes were successfully diffed and returns `nil` (exit code 0 for `harnez diff`).

## Affected Locations

1. **`internal/markdown/markdown.go`** (lines ~147–152, inside `diffSection`):
   - Called by `markdown.Diff` (markdown files like `AGENTS.md`) and `markdown.DiffMK` (Makefiles).
2. **`internal/claude/apply.go`** (lines ~272–278, inside `diffSettingsJSON`):
   - Called by `DiffAll` when diffing `settings.json`.
3. *(Context/Callers)*:
   - `internal/claude/apply.go` line ~42: `diffSectionMD` wraps `markdown.Diff`.
   - `internal/claude/apply.go` line ~688: `DiffAll` invokes `diffSettingsJSON` and `diffSectionMD` via `report(...)`.
   - `cmd/harnez/main.go` line ~112: `diff` CLI command wraps `claude.DiffAll(t, cfg)`.

## CLI Behavior & Exit Codes

- `harnez diff` exit behavior:
  - If identical: exits 0, prints `"No changes."`.
  - If differences found: exits 0, prints unified diff to stdout. (In standard CLI tools like `git diff`, diff output with exit 0 is standard when not using `--exit-code`).
  - If `diff` command fails (exit code ≥ 2 or executable not found): returns error `fmt.Errorf("diff: %w", err)` -> Cobra returns error -> `harnez` exits non-zero (exit code 1).

## Fix / Recommended Patch

Check `cmd.Run()`; distinguish exit code 1 (normal diff output, not an error) from exit code 2 or other execution failures using `errors.As`:

```go
if err := cmd.Run(); err != nil {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		// files differ — expected, not an error
	} else {
		return false, fmt.Errorf("diff %s: %w", label, err)
	}
}
return true, nil
```

### Shared Helper (Optional / DRY)
To avoid duplicating the `exec.ExitError` logic and tempfile management across `internal/markdown` and `internal/claude`, a shared helper (or consistent private helper) can be used. Alternatively, ensure `errors.As(err, &exitErr)` is used consistently in both `diffSection` and `diffSettingsJSON`.

## Edge Cases & Notes

1. **`diff` executable not found in `$PATH`**:
   `cmd.Run()` returns `*exec.Error` (e.g. `exec: "diff": executable file not found in $PATH"`), which will not match `exitErr.ExitCode() == 1` and properly surfaces the error.
2. **Return signature `(bool, error)` on failure**:
   On fatal error, returning `(false, fmt.Errorf(...))` is cleaner so callers don't count a failed diff attempt as `changed = true`.
3. **Signal termination**:
   If `diff` is killed by a signal (SIGKILL/SIGTERM), `exitErr.ExitCode()` returns `-1` (not 1), correctly treating it as a failure.

## Test Strategy

1. **Unit tests (`internal/markdown/markdown_test.go`)**:
   - Test `markdown.Diff` with different contents and verify `(true, nil)` when `diff` binary is present.
   - Test with matching contents and verify `(false, nil)`.
2. **Integration tests (`internal/claude/integration_test.go`)**:
   - Verify `DiffAll` under drift succeeds and outputs unified diff without errors.
3. **Drift / Error test**:
   - Temporarily point `$PATH` to an empty directory or invalid `diff` in a test environment to verify `markdown.Diff` and `DiffAll` return an error instead of returning `nil`.
