# 074 — `harnez distill hook`'s Command Match Is Bypassed by Shell-Wrapper Prefixes

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [[069-pretooluse-hook-optional-autopipe-bash-through-distill]], `internal/distill/hook.go`

---

## 1. Problem

Demonstrated live in session: with `HARNEZ_DISTILL_AUTOPIPE=true`, `go test ./... -v` correctly
triggers the `PreToolUse` rewrite and comes back distilled (just `PASS`/`ok` lines). The
functionally identical `bash -c 'go test ./... -v'` does **not** — all the verbose `=== RUN`/
`--- PASS` noise passes through unfiltered.

Root cause: `internal/distill/hook.go`'s `noisyCommandRE` matches `go\s+test` etc. only at the
start of the literal command string (or right after `;`/`&`/`|`). When the actual command text is
`bash -c '...'`, the regex never sees into the quoted inner command. The same blind spot applies to
`sh -c`, `zsh -c`, and any prefix wrapper (`time go test ./...`, `nice go test ./...`,
`env FOO=bar go test ./...`, `xargs -I{} go test {}`, etc.) — none of these match the allowlist,
so the command silently passes through unrewritten.

This fails safe (no distillation happens, nothing breaks), but it's a real, unrecorded gap in an
already-shipped, closed ticket (069).

## 2. Fix Sketch (not yet decided/implemented)

A reasonably safe fix: unwrap known outer wrappers (`bash -c`/`sh -c`/`zsh -c '...'`, and
`time`/`nice`/`env`-style prefixes) before matching, rewrite the *inner* command, and re-wrap. Care
needed not to mis-parse quoting/escaping inside the wrapped string, and not to over-broaden the
allowlist into false positives for unrelated `-c` usages of arbitrary other binaries.

## 3. Implementation & Verification Plan

1. Extend `internal/distill.RewriteBashCommand` (or a helper it calls) to detect and unwrap
   `bash -c`/`sh -c`/`zsh -c` and common prefix wrappers before matching against
   `noisyCommandRE`, re-wrapping the rewritten inner command in the original wrapper syntax.
2. Add unit tests mirroring `hook_test.go`'s existing cases, specifically for `bash -c '...'`,
   `sh -c "..."`, `time go test ./...`, `env FOO=1 go test ./...`.
3. Verify with a live Bash tool call comparison (as done ad hoc in this session) — direct command
   vs. wrapped command should now both get distilled identically.
