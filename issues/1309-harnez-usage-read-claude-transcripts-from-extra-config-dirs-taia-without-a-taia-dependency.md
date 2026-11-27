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

## Constraints for any option

- Read-only on other config dirs: never write, never read their `settings.json` or credentials. Only `projects/` is used for usage.
- Dedupe by session id across dirs (the same transcript could be reached twice).
- Report rows show their source dir label so taia sessions stay visible as such.
- Tests use fixture dirs only. Never read the real `~/.claude`, `~/.local/state/taia` or real transcripts in tests or helper prompts.
- No `compact_summary` or `custom_instructions` in any output.

## Decision

- Pending owner pick. Host recommendation: A now (spec plus user override), B later only if A's list proves easy to forget.

## Acceptance (for A)

- Fixture test: two config dirs, one shared session id, report counts it once and labels the source.
- Live: `harnez usage history report --window weekly` lists taia sessions. "Not explained locally" drops, and the API $ total rises by the taia sessions' cost. The 5-hour and weekly invariants from #1308 stay exact.
- Hand check: session `d957448b` cost in the report against the `cost-state` total (taia `scripts/session-usage.sh`).
