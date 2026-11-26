# 1304 — harnez apply: one hook entry per agent event, dispatch inside harnez

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: #1303 (adds compaction hook work), #423 (compaction hooks)

---

## Goal

/goal `harnez apply` installs exactly one harnez hook entry per agent event, and that one command decides inside harnez what to do: telemetry, read markers, exec guard, skill tracking, sound and so on. This holds for Claude, and for Codex where it has the same events. Done when:
- the installed settings show one harnez entry per event;
- every current behaviour still works, covered by tests and one live session;
- re-apply is idempotent;
- old multi-entry setups are migrated without leftovers.

Stop and report on an owner decision, for example when an event's matcher cost is the trade-off. On a loop or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

Owner, 2026-11-26: "harnez should only add one hook for each agent event and gracefully manage whatever it needs to do in the hooks itself, instead of setting up multiple hooks."

Claude hooks installed today, from `~/.claude/settings.json`:

| Event | Entries |
|---|---|
| PreCompact | `harnez codex-telemetry`, `harnez hook compact-reset` (2 processes, run in parallel, order undefined) |
| PostCompact | `harnez codex-telemetry` |
| PreToolUse | 6 entries, 4 commands: Bash→`harnez exec hook`; View, Read, ReadMultipleFiles→`harnez hook read`; Edit, Write→`harnez hook pre-edit` |
| PostToolUse, PostToolUseFailure | Skill→`harnez hook skill` |
| UserPromptSubmit | `harnez hook skill` |
| SessionStart | `harnez hook claude-instructions` |
| Stop | `harnez hook sound` |

The Codex hooks are built in `internal/codex/hooks.go:77-90`.

Why one entry per event:
- One place decides order and failure handling. PreCompact telemetry and the read-marker reset no longer race.
- Adding work such as #1303 means no new settings entry, so other sessions need no `apply` and reload. The installed binary carries the change.
- Drift repair and removal in `apply` handle one known entry per event.
- The command names no longer leak internal structure: `codex-telemetry` runs for Claude.

## 2. Findings and trade-offs

- **Matchers save process starts.** Today Claude Code starts harnez only for matching tools. One PreToolUse entry with matcher `*` would start harnez on every tool call. Recommendation: one entry per event whose matcher is the union of the tools harnez handles, e.g. `Bash|View|Read|ReadMultipleFiles|Edit|Write`. Unverified: harnez start-up time per call. Measure it before choosing `*`.
- **Hook output contracts differ per event.** Examples are a PreToolUse decision or rewrite (`exec hook` rewrites Bash input) and SessionStart additional context. The dispatcher must merge the results of several handlers into one valid reply. Define what happens when two handlers both want to block or rewrite.
- **Failure isolation.** One handler's error or slowness must not drop the others. Use a time budget per handler, and get any timeout from `spec/`.

## 3. Plan

- **M1.** Add a `harnez hook <event>` dispatcher with a handler registry. Keep the existing subcommands as handlers. Write tests per event for the merged output and for handler failure.
- **M2.** Make `apply` write one entry per event and migrate old entries. Test idempotency and drift repair (`TestApplyDiffRepairCycle`). Update `docs/CLIDesign.md`.
- **M3.** Do the same for Codex (`internal/codex/hooks.go`).
- **Verify** with a live Claude session: Bash rewrite, a read marker, a compact (telemetry plus reset) and a skill event. Then `make smoke`.
- **Reload note:** after `harnez apply`, every open Claude and Codex session must reload to use the new hook entries. Tell the owner.
