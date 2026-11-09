# Agent Shell — Which Shell Runs an Agent's "Bash" Command

The model writes commands for a tool called Bash, but the shell that runs them depends on the
harness, the user's login shell, and the harnez exec hook. They can differ, and bash and zsh
handle the same text differently.

## Claude Code on a fish login shell (observed 2026-11-09)

```
user terminal      fish      ($SHELL=/bin/fish)
Claude Code        zsh       zsh -c 'source ~/.claude/shell-snapshots/snapshot-zsh-*.sh ... eval <cmd>'
 └─ harnez hook    bash      zsh → ⚙ harnez exec → bash -c '<cmd>'
```

- **Hooks off** (`claude --settings '{"disableAllHooks": true}'`): `ps -p $$` shows `zsh`,
  `ZSH_VERSION=5.9.2`, `BASH_VERSION` is empty, and `SHELL=/usr/bin/zsh`.
- **Hooks on**: the `harnez exec hook` PreToolUse hook (see
  [HookRewritePattern.md](HookRewritePattern.md)) rewrites the command into `harnez exec`, which
  runs `bash -c`. `BASH_VERSION=5.3.20`, and the parent process is `⚙` (harnez exec).
- Before the eval, Claude Code's zsh wrapper sources a shell snapshot (aliases, functions, PATH),
  sets `NO_EXTENDED_GLOB NO_BARE_GLOB_QUAL`, unaliases `which` and `unsetenv`, and afterwards
  writes `pwd -P` to a temp file to carry the cwd forward. The bash child inherits exported
  variables but not zsh aliases or functions.

## How Claude Code chooses the shell (official)

From <https://code.claude.com/docs/en/env-vars>, checked 2026-11-09:

- `CLAUDE_CODE_SHELL`: a path to a `bash` or `zsh` binary. Other shells such as fish are not
  supported. An invalid value falls back to auto-detection.
- Auto-detection uses `$SHELL` when it is bash or zsh, otherwise the first working zsh, then bash,
  on `PATH` and in standard install locations. So a fish user gets zsh before bash.
- `CLAUDE_CODE_SHELL_PREFIX`: wraps Bash tool calls, hook commands, status line commands and stdio
  MCP startup. The wrapper gets the full command as one quoted `$1` and must re-evaluate it
  (`exec bash -c "$1"`).
- `CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR`: returns to the start directory after each command.
- `disableAllHooks` (settings reference): turns off hooks, a custom status line and a custom `@`
  file suggestion command; any settings file, or one session via `--settings`.
- `CLAUDECODE=1` is set in Bash tool and hook subprocesses.

Shell snapshots (`~/.claude/shell-snapshots/`) are not documented officially; issue reports
describe captured aliases, functions and options, and breakage from captured options such as
`set -o onecmd` (unverified).

## Other agents (unverified, from issue trackers)

- Codex CLI runs `/bin/bash` regardless of `$SHELL` (openai/codex#3916).
- Gemini CLI runs `bash -c` on macOS and Linux; a PR to detect zsh was closed unmerged
  (google-gemini/gemini-cli#26912).
- No agent found tells the model which shell actually runs; Claude Code issues report the model
  emitting fish or bash syntax under zsh.

## bash vs zsh differences that bite agent commands

| Case | bash | zsh |
|---|---|---|
| `f="a b"; ls $f` | splits into 2 args | 1 arg |
| `ls *.nomatch` | literal pattern passed | `no matches found`, command not run |
| `arr=(a b); echo ${arr[1]}` | `b` (0-based) | `a` (1-based) |
| `echo 'a\nb'` | literal `\n` | newline |
| `[[ $s =~ re ]]` captures | `BASH_REMATCH` | `$match` |
| `read -a` | array | unsupported (`-A`) |

Rows 2 and 3 are the dangerous ones: they change behaviour without an error.

## Risks and open questions

- **Sessions without the hook** (hooks disabled, other agents, a machine before `harnez apply`)
  run zsh, while hooked sessions run bash: the same command can behave differently.
- **Commands the hook does not rewrite** would run in zsh. Not yet checked.
- **Lost zsh environment**: aliases and functions from the snapshot do not reach `bash -c`.
- To verify with a canary: exit codes and stderr through `harnez exec`; quoting of `"`, `$`,
  backticks and newlines through the double wrap; cwd carry-over; interaction with
  `CLAUDE_CODE_SHELL_PREFIX`.

## Recommendation (not applied)

Set `"CLAUDE_CODE_SHELL": "/usr/bin/bash"` in the `env` block of `~/.claude/settings.json`, so
Claude Code itself runs and snapshots bash and hooked and unhooked sessions agree. If adopted for
all users, ship it through `harnez apply` as a `spec/` value, with the canary above.
