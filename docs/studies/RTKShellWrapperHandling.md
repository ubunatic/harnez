# RTK: Shell-Wrapper and Pipe Handling in Command-Rewrite Hooks

Prior-art reference for issue [[074-distill-hook-bypassed-by-shell-wrapper-prefixes]].
Source: `github.com/rtk-ai/rtk`, default branch `develop`, read directly via `gh api`
(files fetched: `hooks/claude/rtk-rewrite.sh`, `src/hooks/rewrite_cmd.rs`,
`src/discover/registry.rs`, `src/discover/lexer.rs`). All line numbers below refer to
that snapshot; RTK is under active development so exact numbers will drift.

## 1. RTK does command-string interception, via a real Rust binary, not the shell hook

`hooks/claude/rtk-rewrite.sh` is a thin thirty-line delegating script: it reads the
`PreToolUse` JSON payload, extracts `tool_input.command`, and shells out to
`rtk rewrite "$CMD"`. All actual parsing/rewrite logic lives in the Rust binary
(`src/discover/registry.rs`, `src/discover/lexer.rs`), not in the hook script. This
mirrors harnez's split (hook script vs. `internal/distill`), but RTK's rewrite core is
a full quote-aware tokenizer, not a single regexp.

Exit-code protocol (`src/hooks/rewrite_cmd.rs`): 0 = rewritten + auto-allow, 1 = no
RTK equivalent (passthrough), 2 = deny rule matched, 3 = rewritten but force a user
prompt (`Ask`/`Default` permission verdict). Notably `PermissionVerdict::Default` is
deliberately mapped to "ask", not "auto-allow" — the code comment cites
`github.com/rtk-ai/rtk/issues/1155` as the security incident that made this explicit:
auto-allowing any command with a known rewrite, regardless of whether the user had
actually allowed it, was treated as a permission-bypass bug.

## 2. Shell-wrapper prefixes: partially handled, via an explicit allowlist — `bash -c`/`sh -c`/`zsh -c` are NOT unwrapped

`registry.rs` strips a fixed, small set of prefixes before matching, all via
`strip_word_prefix` (literal word-boundary prefix match, not shell parsing):

- **Env-var assignments and `sudo`/`env`** — regex `ENV_PREFIX`
  (`^(?:sudo\s+|env\s+|VAR=value\s+)+`), handling `sudo cmd`, `env FOO=bar cmd`, and
  `FOO=bar cmd` (including quoted values). This *does* cover harnez issue 074's
  `env FOO=bar go test ./...` example.
- **Shell keyword prefixes** (never fall through if inner is unmatched):
  `noglob`, `command`, `builtin`, `exec`, `nocorrect`.
- **A hardcoded routable wrapper**: `uv run`.
- **User-configured `transparent_prefixes`** (`[hooks].transparent_prefixes` in
  `config.toml`), for arbitrary literal prefixes like `docker exec mycontainer` —
  opt-in, not builtin.

None of these lists include `bash -c`, `sh -c`, `zsh -c`, or `time`. `strip_word_prefix`
only strips a literal leading word/phrase followed by a space; it does not parse into a
quoted argument. Even if a user added `bash -c` to `transparent_prefixes`, the "rest"
after stripping would still be the quoted string `'go test ./...'` (quotes included),
which would not match any inner-command pattern — RTK's mechanism is structurally
unable to unwrap a `-c '...'` argument without dedicated support, and no such support
exists in the reviewed source. **Conclusion: RTK has the same blind spot harnez issue
074 describes for `bash -c`/`sh -c`/`zsh -c`/`time`, and does not attempt a
lexer-based unwrap of quoted `-c` arguments.** It does go further than harnez's current
regex for the `env FOO=bar cmd` and `sudo cmd` cases specifically.

## 3. Pipes: RTK rewrites only the pipeline's final stage, and only when that stage is itself a known command — never wraps the whole pipeline

This is the most relevant precedent for harnez's second question. `rewrite_compound`
(`registry.rs`) tokenizes the full command and, on hitting a `|`, calls
`analyze_pipeline` + `rewrite_pipeline_final_stage`:

- Only the **last stage** of a `|` chain is a rewrite candidate
  (`rewrite_pipeline_final_stage` slices `cmd[final_stage_start..end_offset]` and runs
  the normal segment-rewrite on just that slice).
- If the final stage isn't itself something RTK recognizes (e.g. `tee out.log`), the
  pipeline is returned unchanged — RTK never appends its own filter after an unrelated
  final command. Test: `test_rewrite_pipeline_final_normalizes_prefixes` shows
  `cargo test | FOO=1 command grep FAILED` → `cargo test | FOO=1 command rtk grep FAILED`
  (only `grep` was rewritten; `cargo test` upstream of the pipe is untouched).
- RTK explicitly **refuses to rewrite at all** when a pipe is combined with opaque
  grouping (`(...)`/`{...}`) — `has_pipe && has_opaque_grouping → return None` — and
  when the pipe uses `|&` (bash's combined stdout+stderr pipe: `PipeKind::StdoutAndStderr`
  sets `has_supported_structure = false`). Tests: `test_rewrite_opaque_grouped_pipeline_stays_raw`,
  `test_rewrite_stderr_pipe_stays_raw`.
- Separately, `contains_unattestable_construct` (`lexer.rs`) makes the whole command a
  hard passthrough if it contains command substitution (`` `...` ``, `$(...)`) or a
  redirect with a file target (`> file`, but *not* fd-dup like `2>&1`, which still
  rewrites). This is a distinct, even more conservative check than the pipe logic:
  RTK backs off entirely rather than risk misinterpreting quoting/redirection it isn't
  sure it parsed correctly.

**RTK never wraps a whole pipeline in a subshell and appends its own filter at the
end** — the way harnez's `RewriteBashCommand` does today. It rewrites in place, one
segment at a time, and only when it's confident about that segment's shape.

## 4. Stated design philosophy

No standalone philosophy doc was found (README, `hooks/README.md`, and `docs/` were
checked). The closest to an explicit statement is code-level: comments in
`registry.rs` around `contains_unattestable_construct` and the opaque-grouping /
`|&` refusals, plus the `#1155` security-issue comment in `rewrite_cmd.rs`, describe a
consistent bias toward **"when parsing confidence is low, don't rewrite"** rather than
attempting a best-effort transform. There is no comment articulating "respect explicit
shell invocations like `bash -c`" as a deliberate design stance — the `bash -c` gap
looks like an unaddressed case, not a considered exclusion, since RTK does actively
unwrap other explicit wrapper syntax (`sudo`, `env`, `noglob`, `command`, user-configured
`docker exec ...`) when it's confident it can parse the boundary safely. No RTK
issue/discussion specifically about `bash -c` escaping the hook was found in the
reviewed material; this should be treated as unverified rather than "RTK considered and
rejected it."

## 5. Implications for harnez

**On `bash -c` / wrapper prefixes (issue 074's original scope):** RTK's prior art cuts
against "leave it alone by design" as RTK's own stated intent — RTK *does* unwrap
`sudo`/`env`/shell-keyword prefixes, i.e. it treats "explicit wrapper" as something
worth seeing through, not a signal to back off. But RTK also does not attempt the
`bash -c '...'` case specifically, for what looks like a structural reason: unwrapping
a *quoted* inner command require a real shell-lexer's help to find the string boundary
and re-emit it correctly, which is a materially harder problem than stripping a literal
leading word. That supports either project decision: fixing `bash -c` in harnez would
need equivalent lexer investment (harnez's current match is a single regexp, no
tokenizer), or leaving it unrewritten is defensible as "we don't have the parsing
confidence to do this safely yet" — independent of whether `bash -c` is "an explicit
signal to run as-is."

**On the already-shipped pipe behavior (harnez's existing `RewriteBashCommand`):**
traced by hand against `internal/distill/hook.go` (and `hook_test.go`, which has no
existing-pipe-to-a-third-tool test case):

```
go test ./... | tee out.log
→ set -o pipefail; ( go test ./... | tee out.log ) 2>&1 | harnez distill
```

This is safe under harnez's exit-code goal (`set -o pipefail` still applies to the
inner pipeline, so a `go test` failure isn't masked by `tee`'s always-zero exit), and
`tee` still receives the *raw*, unfiltered stdout from `go test` — `harnez distill`
only sees the merged stdout+stderr of the whole subshell (i.e., `tee`'s pass-through
output plus interleaved stderr from both commands) after `tee` has already written the
untouched original to `out.log`. So the shipped behavior does not corrupt `out.log`
and does correctly filter what reaches the agent. This is more permissive than RTK's
approach (RTK would leave `go test ./... | tee out.log` completely untouched, since
`tee` isn't a command RTK knows how to rewrite) but not unsafe by the same criteria RTK
uses (no data loss to the explicit pipe target, no exit-code masking). The interleaving
of stderr from both pipeline stages into one merged stream (`2>&1` outside the subshell)
is the one place harnez's behavior is less precise than RTK's segment-level rewriting,
but it was already the accepted tradeoff from issue 069, not a new finding.
