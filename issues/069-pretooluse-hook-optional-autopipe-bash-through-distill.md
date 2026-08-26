# 069 — PreToolUse Hook: Optional Auto-Pipe of Noisy Bash Commands Through `harnez distill`

**Status**: Closed — resolved
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Observability & Token Efficiency
**Related**: [[066-native-go-command-output-distillation]], [[065-concisemode-caveman-skill-and-output-distillation]]

---

## 1. Problem & Motivation

`harnez distill` (issue 066) exists but is opt-in and manual: an agent has to remember to pipe
noisy commands (`go test ./...`, `git status`, build logs) through it. In practice that never
happens reliably — the agent forgets, or the command is buried in a larger tool call.

We want an automatic path: a Claude Code `PreToolUse` hook on the `Bash` matcher that rewrites the
command to pipe through `harnez distill` before it runs, so noisy output never reaches the
transcript in the first place. This must be **optional** — never force distillation on every Bash
call unconditionally, since that risks mangling interactive commands, exit codes, or commands the
user actually wants raw.

## 2. Research: What Claude Code Hooks Can Actually Do

Confirmed via `claude-code-guide` research against the official hooks reference
(`https://code.claude.com/docs/en/hooks.md`):

- **`PostToolUse` cannot rewrite tool output.** The tool has already run; a `PostToolUse` hook can
  only append `additionalContext` (a system-reminder-style note), not replace what the model sees.
- **`PreToolUse` *can* rewrite tool input** via `hookSpecificOutput.updatedInput` before the tool
  executes. This is the only viable mechanism for this feature.
- Hooks are registered in `settings.json` under `hooks.PreToolUse[].matcher` (`"Bash"`) — global
  only via `harnez apply` (`~/.claude/settings.json`), consistent with this repo's existing
  `apply`/`init` scope split (`docs/CLIDesign.md`).

## 3. Design

1. **Opt-in via env var, not install-time**: the hook is installed globally (harmless — near-zero
   overhead when disabled) but only rewrites commands when `HARNEZ_DISTILL_AUTOPIPE=true` is set.
   Default `"false"` in `config.yaml`'s `env:` block, same pattern as `DEBUG`.
2. **Allowlist, not denylist**: only rewrite commands matching a curated set of known-noisy,
   known-safe-to-pipe patterns (`go test`, `go build`, `go vet`, `cargo test`/`build`, `pytest`,
   `npm test`, `make test`/`build`/`check`, `git status`/`diff`/`log`). Anything else — editors,
   `ssh`, `docker exec -it`, servers/watchers, already-piped commands — passes through unchanged.
   A denylist-of-interactive-commands approach was rejected: too easy to miss a case and silently
   mangle an interactive session.
3. **Preserve exit-code semantics**: naively appending `| harnez distill` makes `$?` reflect
   `distill`'s exit code (always 0), hiding test/build failures from the agent. Wrap as
   `set -o pipefail; ( <original command> ) 2>&1 | harnez distill` so `$?` still reflects the
   original command's failure.
4. **Avoid double-piping**: skip rewriting if the command already contains `harnez distill`.
5. Implement the matching/rewrite logic in `internal/distill` (testable), with a thin `harnez
   distill hook` subcommand doing stdin/stdout JSON plumbing for the actual hook invocation.

## 4. Implementation & Verification Plan

1. `internal/distill`: add `RewriteBashCommand(command string) (rewritten string, ok bool)` with
   unit tests covering the allowlist, pipefail wrapping, and double-pipe guard.
2. `cmd/harnez/distill.go`: add `harnez distill hook` subcommand — reads the `PreToolUse` JSON
   payload from stdin, checks `HARNEZ_DISTILL_AUTOPIPE`, emits `updatedInput` JSON when rewriting,
   exits 0 with no output otherwise.
3. `config.yaml`: register the hook (`event: PreToolUse`, `matcher: Bash`, `command: harnez distill
   hook`) and add `HARNEZ_DISTILL_AUTOPIPE: "false"` to `env:`.
4. Verify with unit tests, a manual `harnez apply` + hook invocation smoke test, and `harnez
   status`.
