# Databases

Every place where harnez stores data: databases, state files, logs and caches. Paths use the
defaults; `$XDG_STATE_HOME`, `$XDG_CONFIG_HOME` and `$XDG_RUNTIME_DIR` move the XDG ones.
Inventory from 2026-09-25 (codex luna:med scan, host-verified against the code and this machine).

## Databases (SQLite)

| Path | Written by | Holds | Read by |
|---|---|---|---|
| `~/.harnez/tool_catalog.sqlite` | `internal/telemetry/telemetry.go` | Tool-call telemetry and classifications (hooks) | `hook`, `stats`, `usage export` |
| `~/.harnez/bench/bench.sqlite` | `internal/bench/store.go` | Bench runs: table `runs` with response, score detail, tokens, cached, order | `bench run`, `bench results` |

`~/.harnez/bench/enabled` is the marker file that turns `harnez bench` on (`internal/bench/setup.go`).

## Logs and history (JSONL)

| Path | Written by | Holds | Read by |
|---|---|---|---|
| `~/.harnez/agymeter/usage.jsonl` | `internal/agymeter/meter.go` | agy proxy usage per call (prompt, cached, model); the dir also has the meter's TLS key and certs | `stats`, `bench` (agy tokens) |
| `~/.claude/harnez/usage-history/*.jsonl` (+ `.lock`) | `internal/usage/history.go`, `quota_history.go` | Quota history per host and agent; copy files from other machines here | `stats`, `usage` |
| `~/.harnez/feedback/*.jsonl` | `internal/feedback/feedback.go` | Agent and user feedback per project, with promotion status | `feedback list`, `feedback promote` |
| `<repo>/.git/harnez/quota-1-logs/` | `cmd/harnez/exec.go` | Output of each quota test run | the agent after a `make test-q1` |

## State and caches (JSON, text)

| Path | Written by | Holds |
|---|---|---|
| `~/.harnez/agents/` | `internal/subagent/session.go` | `harnez agent` session records and registry lock |
| `~/.harnez/sessions/` | `internal/sessionstate`, `internal/resolve` | Per-session state: last command, usage, ticket link, read-hook state, locks |
| `<repo>/.git/harnez/quota_1.state` (outside git: `<dir>/.harnez/`) | `internal/quota1/quota.go` | Time of the last quota test run and the tree state it ran on |
| `~/.local/state/harnez/agents/usage/*.json` | `internal/usage/statecache.go` | Cached provider usage and refresh times |
| `~/.local/state/harnez/fetch-durations.json` (+ `.lock`) | `internal/usage/fetchdurations.go` | How long each usage fetch takes, for timeouts |
| `$XDG_RUNTIME_DIR/harnez/procs/` (else `~/.harnez/run/procs/`) | `internal/procs/records.go` | Records of running harnez processes |
| Provider trees (`~/.claude`, `~/.codex`, `~/.gemini`) | provider CLIs; harnez adds quota caches (`internal/usage/agy.go`) | Read for usage and sessions; harnez-owned files there are only caches |

## Config and generated files

| Path | Written by | Holds |
|---|---|---|
| `~/.harnez/config.yaml` | user, `harnez apply` | Global config, e.g. `reading_discipline.enforce` |
| `~/.config/harnez/local.yaml` | user | Machine-local overrides (`usage:`, `load:`) |
| `~/.harnez/env.sh`, `~/.harnez/shims/bash` | `internal/claude/apply.go` | Shell env and bash shim used by agent hooks |
| `~/.harnez/debug.log`, other `*.log` | `cmd/harnez/exec.go` and features | Diagnostics |

## Temporary

`/tmp/harnez-*` holds per-command scratch: rendered PNG cards, bench workspaces (`harnez-bench.*`,
kept on purpose so cards can be opened), diffs and load-control files. Not durable; safe to delete
when no harnez command runs.

## Leftovers

- `~/.harnez/telemetry.db` and `telemetry.sqlite` are empty and no production code opens them;
  only tests name them, so a test probably writes into the real home. Safe to delete.
- `~/.harnez/tool_catalog.sqlite.bak-issue331-*` is a one-off backup from issue 331.
