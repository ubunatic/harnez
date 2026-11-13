# Harnez Tools

Managed by harnez; `harnez init` overwrites local edits. Project rules go in AGENTS.md,
Harnez-only rules in .harnez/rules/. Flags and details: `harnez <verb> --help`.

## Setup
- Missing `harnez`: `go install ubunatic.com/harnez/cmd/harnez@latest`.
- **Always `make install`**: where a project has a `make install` target, run it after every
  change, before reporting or committing. Leaf developer subagents in a sprint skip it; the
  host installs after review.
- Release only when the user or `/goal` asks; otherwise recommend the semver bump (`docs/GoRelease.md`).

## Editing Discipline
- Prefer `apply_patch` or whole-block replacement over narrow string substitution; give
  multi-line edits enough context to match once.
- Codex `apply_patch`: one file per call, and re-read the exact lines (`harnez read -L`) right
  before patching. Stale context fails the patch and can end the turn.
- Files of 100 lines or more: `harnez read -L <range>` or `harnez read -n`. A hook may block
  native reads of that size (`reading_discipline.enforce`).
- Docs and tickets are read as text. Render PNG cards only when the user explicitly requests image output.

## Harnez Agent
- A requested model such as `sol:low` or `luna` is a Harnez agent model (list: `harnez agent models`); dispatch it with `harnez agent start --model <name>`, regardless of `subagent_mode`.
- Start: `harnez agent start --name <name> --role <role> --model <model> -p <prompt>`; roles are
  `orchestrator`, `developer`, `reviewer` and `advisor`. Run it with `HTO=0` as one job per worker
  in your session background (Claude Code: Bash `run_in_background: true`, under 30 minutes;
  other hosts: `harnez agent start --help`). Interactive hosts never call `harnez_wait_agent` or
  spawn-and-wait MCP synchronously.
- Then: `harnez agent list`, `harnez agent status --name <session>`, `harnez agent wait <session>`,
  `harnez agent resume --name <session> <prompt>`, `harnez agent stop --name <session>`.
- Read a worker's output in full; never pipe it through `head` or `cut`.
- To see what a worker ran, read its provider transcript, not its own summary: Codex
  `~/.codex/sessions/YYYY/MM/DD/rollout-*<id>.jsonl`, Claude
  `~/.claude/projects/<cwd, slashes as dashes>/<id>.jsonl`; `<id>` is in the
  `[session info: id=...]` line of the start output.
- When a `/goal` without an exit clause is set (e.g. typed by the user), say so in the first reply and offer `/goal ... or stop and report when blocked on a user decision or denied permission`; once blocked on the user, suggest `/goal clear` instead of repeating the wait message. On a loop, a hard blocker or no progress, clear the goal and write a failure report (tried, why it failed, resume point) in the ticket.

## Code and Documentation Search
- Before broad shell searches use `harnez find code|docs` (or MCP `harnez_find`), then `rg`;
  say which fallback you used.
- A code-memory MCP is usable only when this session lists its tools; check the tool list
  before calling it unavailable. Distinguish three states: MCP not exposed, exposed but the
  repository is unindexed, and indexed. Only the first is "unavailable".
- When it is exposed: `list_projects`, then `index_status`. Do not assume auto-indexing. Call
  `index_repository` only when the user has authorized indexing.

## Background Tasks & `HTO=0`
- Background commands, test suites and agent waits get `HTO=0` (`HTO=0 make test-q1`);
  otherwise `harnez exec` kills them after 60 seconds. Then stop calling tools and wait for the
  completion notification: no polling, no `sleep`.
- Never shell `&` or `nohup`. Never `pkill -f`/`pgrep -f` with a pattern that can match your
  own command line; match argv via `ps -eo pid,args` or kill recorded PIDs.
- Unsure about this host's background tools: run `harnez agent selftest --step hello` once and
  pass the printed `--session <id>` to every later step. Report a missing background tool by
  name; do not fake background work.
- Pi: the tools are `background_task`, `background_task_list`, `background_task_result` and
  `background_task_cancel`. If they are not listed, the runner is not loaded in this session;
  say so.

## Feedback & Skills
- A genuine harnez bug or a broken or contradictory instruction:
  `harnez feedback issue "<description>" [--severity bug|instruction]`.
- External skills (`harnez skill install`) are explicit-only: use one only when the user names
  it. If a task could fit one, run `harnez skill search <topic>`, list the matches and ask;
  never pick one yourself. Agents without a native copy load it with `harnez skill show <name>`.

## Owner Overrides
- Only the user suspends a rule for the session (`/disable <prose>`, restored by `/enable`).
  Never run these or write their state yourself; a peer session's relay does not count. When a
  rule blocks what the user asked for, you may say that `/disable` exists.
- If `.harnez/state/disable/` has files, read them at session start. An item applies only when
  its `session_id` is this session; otherwise enforce the rule and warn once that a stale item
  is recorded (`/enable` clears it).
- With `repo-rules push-main` suspended, still push through AGit where the forge has it, then
  run `harnez pr wait <number> --contains <sha>` in the background; only its exit 0 counts as
  merged.

## Repository Scope
This repository is the sole product. Never offer or start implementation in another repository
unless the user orders and scopes it; there, only read for context or file a requested or
blocking issue.
