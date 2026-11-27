# 1309 — harnez usage: read Claude transcripts from extra config dirs (taia) without a taia dependency

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: #1300 (Q2, Q3, Q5), #1302 (report, attribution), #1308 (5-hour share), taia handoff `.harnez/inbox/2026-11-27-claude-pm-handoff-430-usage.md` (in the taia repo)

---

## Goal

/goal `harnez usage` (the report, `usage` totals and the transcript lookups) counts Claude sessions that ran under another `CLAUDE_CONFIG_DIR`, such as taia's, with no import of, call to or path hard-coded for taia, and taia needs no harnez change. Done means: the weekly report lists taia-run sessions with their source, no session is counted twice, the "not explained locally" share drops by what those sessions used, and nothing is written to the other config dir. Stop and report when the owner has not picked an option, or when the other dir's transcripts turn out not to share Claude's layout. On a loop or no progress, clear the goal and write a failure report.

## Problem (live, 2026-11-27)

- taia starts Claude with `CLAUDE_CONFIG_DIR=~/.local/state/taia/claude` (taia `internal/agent/claude.go:169`, `internal/cli/claudebox.go:157`). Its transcripts sit in `~/.local/state/taia/claude/projects/<slug>/<id>.jsonl`, the same layout as `~/.claude/projects`.
- harnez reads only `~/.claude`:
  - `cmd/harnez/usage_report.go:73` hard-codes `~/.claude/projects` for the report.
  - `internal/usage/claude.go:283` defaults `CollectClaude` to `~/.claude`.
  - `internal/transcript/commands.go:37` and `cmd/harnez/hook.go:948` honour `CLAUDE_CONFIG_DIR`, but only that of the current process.
- The taia dir has 40 transcripts (70.8 MB) changed since the weekly reset 2026-11-21 11:00 UTC, 74 in all, across projects including harnez itself. Example: session `d957448b` ($1.91 Haiku, 160 calls, per its own handoff) is missing from the report. Only its `cli_invocations` row reached telemetry.
- The quota is per account, so these sessions raise the weekly and 5-hour quota while the report books them as "not explained locally" (14 of 57 pp this week). The share that is really taia is unknown until the dirs are read.

## Options

| # | Option | taia coupling | Notes |
|---|---|---|---|
| A | **Spec list of extra Claude config dirs**: `usage.claude_config_dirs` in `spec/usage.yaml` (default `[~/.claude]`), user override in the harnez user config. All readers loop over it. | None: taia's path appears only in the owner's config | Explicit, testable with fixture dirs. Recommended. |
| B | **Auto-learn dirs**: hooks and `harnez exec` already see `CLAUDE_CONFIG_DIR`. Record each new value in telemetry, and readers add the learned dirs. | None | No config. It misses sessions where harnez never ran, and a stale dir lingers. Good as a later add-on to A (suggests the entry). |
| C | `--claude-dir` flag (repeatable) or `HARNEZ_CLAUDE_DIRS` env on `usage` commands | None | Quick. It must be passed every time, and the collector and statusline need it too. |
| D | **Symlink** taia's `projects/*` into `~/.claude/projects` | None in code | No code, but it mixes dirs that Claude Code itself manages and risks slug collisions. Not recommended. |
| E | **OpenTelemetry**: Claude Code's own `OTEL_*` export from taia sessions into a harnez OTLP receiver | Needs taia to pass env through | Live data without reading files, but a large new receiver and a taia change. Out of scope. |

## Assessment: hooks in confined taia sessions vs. reading files on the host (2026-11-27)

The owner's constraint: taia sessions run confined, and taia's hooks must not call into `harnez usage`. If hooks are added, they must work under any confinement, for example by talking to the collector over the network.

What each data source needs:

| Data | Source today | Needs a hook in taia? |
|---|---|---|
| Tokens, cost, calls, model, per-session attribution | Transcript JSONL, read by harnez on the host | No. The confined Claude writes to `~/.local/state/taia/claude`, a host dir bound read-write into the sandbox (taia `claudebwrap.go`, `--bind m.Config`). harnez reads it from outside. |
| Weekly and 5-hour quota samples | The host collector polls `api.anthropic.com/api/oauth/usage` (`internal/usage/claude.go:19`). The quota is per account. | No. taia's use already shows in these samples; only its attribution is missing. |
| Compaction point (`pre_tokens`) | `compact_boundary` in the transcript | No. It is the same file. |
| Live statusline, session-boundary rows, `tool_calls` | Claude hooks and statusline | Yes, but taia runs Claude headless (stream-json), so there is no statusline. Nothing the report needs. |

Hook transport, if it is ever wanted:
- **Today:** the collector listens only on a Unix socket (`spec/server.yaml` `socket_path: server.sock`). A confined process reaches it only through harnez's own runner broker FD (`internal/server/runner_channel.go:187`) or a bind of the socket dir. taia has neither, and adding either would tie taia to harnez.
- **Network:** taia's bwrap does not unshare the network (`claudebwrap.go:30`), so a localhost TCP or HTTP endpoint would be reachable from any confinement that keeps network access. Costs:
  - a new listener;
  - an auth token that must be passed into the sandbox, where the agent can read it;
  - any local process could inject telemetry rows.
- **Coupling:** the hook still has to live in taia's Claude config. It is loose only if the hook is a plain HTTP POST of the hook payload, so that no harnez binary runs in the sandbox. Unverified: whether Claude Code supports HTTP-type hooks in the installed version. Check `claude --version` against the hooks docs before relying on it.

**Verdict:** option A needs no hooks, no network and no harnez code in taia, and confinement does not matter to it. Hooks over the network would add only live-only rows that the report does not use, at the cost of a new authenticated listener. Recommendation: A. Revisit network hooks only if a live, per-turn view of taia sessions becomes a goal (its own ticket).

## Constraints for any option

- Read-only on other config dirs: never write, never read their `settings.json` or credentials. Only `projects/` is used for usage.
- Dedupe by session id across dirs (the same transcript could be reached twice).
- Report rows show their source dir label so taia sessions stay visible as such.
- Tests use fixture dirs only. Never read the real `~/.claude`, `~/.local/state/taia` or real transcripts in tests or helper prompts.
- No `compact_summary` or `custom_instructions` in any output.

## Decision

- Owner picked **A** (2026-11-27). B (auto-learn) stays out of scope.
- Lean sprint. Developer `hydrogen:med` (owner's pick: the Claude weekly quota was at 95%), one review agy:neon:low.
- No Searcher: the ticket cites the locations, and the developer's first turn is read-only and confirms the write set.
- The spec default stays `[~/.claude]`. taia's path goes only into the owner's user override, never into the repo spec, tests or docs.
- Quota fetch and credentials always come from the primary dir only. Extra dirs contribute only `projects/*.jsonl`.

## Milestones

### M1: config list and the report reader
- Spec key, for example `usage.claude_config_dirs` in `spec/usage.yaml`, with its JSON schema. Use the repo's existing user-override mechanism for spec values: find it in the plan turn and name it. Expand `~`.
- `harnez usage history report` scans `projects/` in every listed dir. Rules:
  - Dedupe by session id. On a collision, the first dir in list order wins, deterministically.
  - A missing or unreadable dir is skipped with a note, not an error.
  - Each session row carries its source label in the JSON (`source`) and in the text output.
- Every other reader that hard-codes `~/.claude/projects` for usage totals goes through the same list. The plan turn lists them; do not change reads of `settings.json` or credentials.
- Tests use fixture dirs under `t.TempDir()` and a fake HOME. Cases:
  - two dirs that share a session id;
  - a missing dir;
  - an empty list falling back to the default;
  - `~` expansion;
  - subagent transcripts in nested dirs still excluded.

  No test reads the real `~/.claude` or `~/.local/state/taia`.

### M2: docs and live acceptance
- `docs/Telemetry.md` (the report section) and the spec docs name the key and the override. The host runs the live acceptance below with the owner's override set.

## Acceptance (for A)

- Fixture test: two config dirs, one shared session id, report counts it once and labels the source.
- Live: `harnez usage history report --window weekly` lists taia sessions. "Not explained locally" drops, and the API $ total rises by the taia sessions' cost. The 5-hour and weekly invariants from #1308 stay exact.
- Hand check: session `d957448b` cost in the report against the `cost-state` total (taia `scripts/session-usage.sh`).
