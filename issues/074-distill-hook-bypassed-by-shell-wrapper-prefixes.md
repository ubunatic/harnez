# 074 — `harnez distill hook`'s Command Match Is Bypassed by Shell-Wrapper Prefixes

**Status**: Closed — won't fix (documented as intentional, `internal/distill/hook.go` comment + `docs/studies/RTKShellWrapperHandling.md`)
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [[069-pretooluse-hook-optional-autopipe-bash-through-distill]], `internal/distill/hook.go`,
[[RTKShellWrapperHandling]]

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

## 2. Prior-Art Research (RTK) & Owner's Lean

See [[RTKShellWrapperHandling]] for the full writeup. Summary relevant to this ticket:

- RTK ("Rust Token Killer", the project's stated inspiration for `harnez distill`) does its own
  command-string interception via a real quote-aware tokenizer, not a plain regexp, and it *does*
  unwrap some explicit wrapper prefixes before matching: `sudo`, `env FOO=bar cmd`, bare
  `FOO=bar cmd` assignments, and shell keywords (`noglob`, `command`, `builtin`, `exec`,
  `nocorrect`), plus user-configured literal prefixes (e.g. `docker exec mycontainer`).
- RTK does **not** unwrap `bash -c '...'` / `sh -c` / `zsh -c` / `time` — the same gap this ticket
  describes. Its prefix-stripping only removes a literal leading word; unwrapping a *quoted* `-c`
  argument would need dedicated shell-lexer support that isn't present in the reviewed source. No
  RTK issue/discussion specifically weighing "should `bash -c` be unwrapped" was found — this looks
  like an unaddressed case in RTK too, not a considered design exclusion, since RTK is otherwise
  willing to see through other explicit wrappers.
- Separately, tracing harnez's *already-shipped* pipe handling (`go test ./... | tee out.log` →
  `set -o pipefail; ( go test ./... | tee out.log ) 2>&1 | harnez distill`) found it safe: `tee`
  still receives the raw, unfiltered `go test` output before the merged stream reaches
  `harnez distill`, and `set -o pipefail` still surfaces a `go test` failure through `tee`'s
  always-zero exit code. This is *more permissive* than RTK, which would leave that whole pipeline
  untouched (RTK only ever rewrites a pipeline's last stage, and only if it recognizes that stage —
  `tee` isn't in its registry). No change needed here; recorded as a traced-through confirmation,
  not a bug.

**Project owner's stated lean** (2026-08-26): this may not be a bug to fix. `bash -c '...'` is
arguably an *explicit* signal that the command should run as-is — e.g. already piping its own
output elsewhere — and rewriting inside a quoted `-c` string, or wrapping a command that already
manages its own pipe, risks breaking that intent rather than just filtering noise. RTK's own
behavior does not resolve this either way (see above) — it's a genuine open design call, not one
where prior art dictates an answer.

## 3. Fix Sketch (two options — decision pending)

**Option A — fix it.** Unwrap known outer wrappers (`bash -c`/`sh -c`/`zsh -c '...'`, and
`time`/`nice`/`env`-style prefixes) before matching, rewrite the *inner* command, and re-wrap.
Tradeoffs: requires real quoting-aware parsing (a plain regexp isn't enough, per RTK's experience);
risk of mis-parsing escaped/nested quotes inside the wrapped string; risk of over-broadening into
false positives for unrelated `-c` usages of other binaries; and it means harnez decides it should
see through what may be a deliberate "run this exactly as I wrote it" signal from the command's
author.

Implementation plan if chosen:
1. Extend `internal/distill.RewriteBashCommand` (or a helper it calls) to detect and unwrap
   `bash -c`/`sh -c`/`zsh -c` and common prefix wrappers before matching against
   `noisyCommandRE`, re-wrapping the rewritten inner command in the original wrapper syntax.
2. Add unit tests mirroring `hook_test.go`'s existing cases, specifically for `bash -c '...'`,
   `sh -c "..."`, `time go test ./...`, `env FOO=1 go test ./...`.
3. Verify with a live Bash tool call comparison (as done ad hoc in this session) — direct command
   vs. wrapped command should now both get distilled identically.

**Option B — leave it, deliberately.** Treat `bash -c`/`sh -c`/`zsh -c` and other wrapper prefixes
as an intentional escape hatch: a command author who reaches for `bash -c '...'` gets exactly what
they wrote, unfiltered, on purpose. Fails safe today (issue 069's original framing) with no risk of
misparsing quoted content. Tradeoff: the gap stays inconsistent-looking (`go test ./...` gets
distilled, `bash -c 'go test ./...'` doesn't) unless documented as intentional — e.g. a one-line
note in `internal/distill/hook.go`'s doc comment and/or `docs/` explaining that wrapper prefixes are
treated as an explicit opt-out, so a future agent doesn't rediscover this as an unrecorded bug
again.

**Decision (2026-08-26)**: won't fix — Option B. Wrapper prefixes (`bash -c`, `sh -c`, `zsh -c`,
`time`, `env FOO=bar`, ...) are treated as an explicit opt-out from auto-piping, not a gap.
Documented in `internal/distill/hook.go`'s `noisyCommandRE` doc comment.
