# 551 — exec: infer tool name and quota-1 detection through /bin/bash -c and -lc

**Status**: Open
**Priority**: P2
**Severity**: Low
**Category**: Bug / Telemetry
**Related**: [[550-harnez-apply-installs-harnez-agy-launcher-in-local-bin-to-shim-agy]], [[537-agy-route-shell-commands-through-harnez-exec-via-hooks-json]]

---

## Problem

agy runs every command as `bash -c '<program>'`; the shim turns that into
`harnez exec -- /bin/bash -c '<program>'`. `inferToolFromArgs` (cmd/harnez/exec.go ~435) only unwraps
`bash`/`sh` with `-c`, so `/bin/bash` (and `-lc`) is not unwrapped and shimmed rows are likely named
`bash` instead of `go`/`make`. Unverified: whether quota-1 detection of test commands has the same gap.

## /goal

Tool names and quota-1 detection are the same for `bash -c X`, `/bin/bash -c X`, `sh -lc X` and a
direct `X`. Check telemetry first to confirm the gap.
