# Jev Compaction in Agent Harnesses

How harnez replaces an agent's own compaction summary with `harnez compact` (Jev, verbatim
transcript fitting), per harness. Consult before touching `internal/claude/jevcompaction/`,
`spec/jev_compaction.yaml`, or any compaction hook. History: issues 635, 644, 646;
background study: [studies/FastJevCompation.md](studies/FastJevCompation.md).

## Which harness allows a real replacement

| Harness | Replace the summary live? | Route |
| --- | --- | --- |
| Claude Code 2.1.285+ | **Yes** | `session.compact` function hook returns `{messages}` |
| Codex 0.159.1 | No | `PreCompact`/`PostCompact` are notify-only; `compact_prompt` / `experimental_compact_prompt_file` only affect local (not provider-side) compaction |
| agy 1.2.13 | No | no compaction hook event at all; `PreInvocation` can inject, not rewrite |

Remaining routes for Codex/agy (each needs its own canary, see 646): a better compaction
prompt (Codex only), or rewrite the session on disk and resume it.

**Why the standard Claude `PreCompact` command hook does not work:** it can only observe. It
cannot return replacement messages, and Claude summarizes its in-memory history, so rewriting
the transcript file has no effect on the live session.

## Claude plugin (issue 646)

- Opt-in: `jev_compaction_enabled: true` in `config.yaml`, then `harnez apply`. Pitfall: apply
  reads the `config.yaml` embedded in the binary, so a repo edit needs `make install` before
  apply (developers only); release users must pass a full config copy with `-c` until issue 764
  adds a user-level setting. Apply writes
  the embedded plugin (marketplace + cache under `~/.claude`), enables it and sets
  `CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1`. Setting it back to `false` removes all of it; the
  `harnez-local` marketplace entry is removed only if it points at harnez's own directory.
- All TS lives in the Go tree (`internal/claude/jevcompaction/`, `go:embed`). No copy outside
  the binary. Thresholds (auto-trigger %, minimum reduction, pinned messages, timeout, size
  limit) come from `spec/jev_compaction.yaml` and are rendered into `hook.ts.tmpl`.
- Flow: `session.compact` gives `trigger` and `messages` (`role`, `text`, `toolUses`, `handle`),
  but no session id or transcript path. The hook pipes the messages as JSONL to
  `harnez compact` over stdin, maps the result back by message and tool index, and returns it.
  On error, truncated output or too little reduction it logs why and calls `next(event)` (the
  built-in summary). `turn.complete` triggers compaction at the configured context share.
- Measured (manual `/compact`, Haiku): 667,780 -> 393,146 JSON bytes (-41%), context
  26,292 -> 10,632 tokens. Auto-trigger is not yet verified in a real long session.

## Function-hook runtime pitfalls

See [ClaudeFunctionHooks.md](ClaudeFunctionHooks.md): import and `$` rules, the only
bridge (`$.process.run`), and why a mistake registers zero hooks silently.

## Canaries and testing

- `scripts/canary-646-claude-compact.sh` starts an interactive Claude with the canary plugin in
  a scratch dir (your `~/.claude/settings.json` untouched); `... check` greps the saved session
  for `compact_boundary` and the marker.
- `canary/646-session-compact/run.sh` (M1, scripted) and `m2-run.sh` (real `harnez compact`).
- Never give a canary a fresh `CLAUDE_CONFIG_DIR`: it has no login, and credentials must not be
  copied. Use the real config with per-process flags (`--plugin-dir`, env).

## Related: unknown context size (issue 644)

agy reports only a turn total, not the context size. harnez stores `ContextTokens = -1`
(unknown); auto-compaction and the threshold check skip unknown values instead of treating the
turn total as context, and JSON output omits the field.

### Native agy compaction (issue 823)

agy 1.3.5+ has its own `/compact` command; this summarizes the conversation and
does not replace its summary with Jev. Use `harnez agent compact --name <session>`
or `harnez agent --name <session> -p /compact` for on-demand compaction. The batch
driver resumes the provider conversation with `--output-format json -p /compact`,
inheriting its saved model and effort.

Verified on agy 1.4.2 with Gemini 3.8 Flash at low effort: print mode saves a
summary step even though JSON reports an empty successful response and stderr
warns that the run ended with no output. Usage is unchanged and provides no
post-compaction context size or reduction. Automatic threshold compaction stays
disabled until a reliable current-context count is available. The interactive
control path writes `/compact` to the PTY but was not verified live in this canary.
Exact commands and saved-summary evidence are in issue 823; rerun the small probe
with `python3 scripts/canary-823-agy-compact.py`.
