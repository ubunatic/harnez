# diff exit code 2 (error) silently swallowed

**Severity:** Medium — false success reporting when diff fails

## Problem

Both `diffSectionMD` and `diffSettingsJSON` call `cmd.Run()` and drop the return value:

```go
cmd := exec.Command("diff", "-u", "--label", label, "--label", label, oldFile, newFile)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
cmd.Run()        // error ignored
return true, nil
```

`diff` exit codes: 0 = identical, 1 = files differ (normal), 2 = error (missing binary,
unreadable file, etc.). Dropping the error means a `diff` crash returns `true, nil` —
claiming output was shown when nothing was printed.

**Affected:** `apply.go` ~line 130 (`diffSectionMD`), ~line 478 (`diffSettingsJSON`)

## Fix

Check the error; distinguish exit code 1 (normal diff output, not an error) from exit code 2
(actual failure):

```go
err := cmd.Run()
if err != nil {
    if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
        // files differ — expected, not an error
    } else {
        return true, err
    }
}
```
