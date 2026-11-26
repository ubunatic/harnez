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

## Decision (decide mode, 2026-11-26)

- **Order.** Runs before #1303, after #1302 is merged. Branch `sprint/1304` from main. Sequential in this checkout, with no worktree.
- **Developer: `claude:haiku:high`, plan-only first turn.** Reason: merging handler replies under per-event hook output contracts, plus migrating existing settings entries in `apply`, has non-obvious failure modes. A wrong merge blocks or rewrites tool calls in every session.
- **Review.** A one-shot `agy:neon:med` reviewer after M1 (dispatcher) and after M2 (apply migration). M3 (Codex) is smaller, and the host reviews it alone unless its diff exceeds about 300 lines.
- **Before M2 lands:** `make smoke` and a scratch-HOME apply → re-apply → drift-repair run. After merge, the owner reloads every open session; tell them.

General rule for haiku:high over haiku:med (owner asked, decide mode):
- **Use high** when the milestone changes an external contract: hook replies, settings/config migration, a concurrency or lock path. Also when an unspecified design choice must be made inside the code, or when med needed two rounds on the same defect class.
- **Use med** for work with complete pre-work: report or query logic, fixtures, docs, wiring.
- Cost: high fills context faster. The Haiku compaction threshold on this machine is 200k, and the >100k tier costs 5x, so use high only where it pays off.

## Plan turn review (host)

The dev-1304 (haiku:high) plan turn mapped the wiring and handlers with file:line. Its key findings:
- There are 14 Claude entries today: `config.yaml:326-366`, plus compaction in `spec/claude-compaction.yaml`.
- `removeHarnezHooks` (`internal/claude/apply_settings.go:117-162`) already strips every `harnez …` command before re-adding, so migration mostly comes for free. Commands with an absolute-path `harnez` are not matched.
- Today exactly one JSON-writing handler runs per tool, so a merge conflict cannot happen yet.
- Start-up time, measured with `printf '{}' | harnez hook skill` over 20 runs: median 37.7 ms, p90 39.8 ms. This is a lower bound.

Checked against https://code.claude.com/docs/en/hooks:
- A matcher of only letters, digits, `_`, `-`, spaces, `,` and `|` is an exact list, so `Bash|View|Read|ReadMultipleFiles|Edit|Write` is valid.
- Matching hooks run in parallel.
- The default command timeout is 600 s, and each entry can set its own with `timeout`.
- PreToolUse precedence is `deny` > `defer` > `ask` > `allow`. `updatedInput` replaces the whole input.
- SessionStart plain-text stdout becomes context.
- Not verified: JSON `additionalContext` on SessionStart, and dedup of identical entries within one file.

## Decision (decide mode, plan review)

- **Q1 matcher.** Use the union list, not `*`. Reason: `*` adds about 38 ms to every non-harnez tool call, and an exact list is a documented matcher form.
- **Q2 order.** Handlers of one event run sequentially, in registry order (a slice). Reason: this removes today's undefined PreCompact order. Every handler is fast except `pre-edit`, which is the only one on Edit and Write.
- **Q3 budget.** `spec/hooks.yaml` `handler_timeout: 40s`, with a schema. It guards against hangs and is not a latency target. It must exceed the largest existing inner budget, the jev 30 s in `spec/decide.yaml:16`. No settings-entry `timeout` is written; Claude's 600 s default stays.
- **Merge.** For PreToolUse decisions, follow Claude's precedence `deny` > `defer` > `ask` > `allow`, and keep the reason of the winning handler.
  - At most one `updatedInput`. A second rewrite is dropped and logged to stderr.
  - `additionalContext` and `systemMessage` strings are joined with `\n` in registry order.
  - SessionStart stays plain text. The registry rejects a JSON handler and a text handler on the same event, enforced by a test.
- **Failure isolation.** Each handler runs under the budget with panic recovery. An error, timeout or panic drops only that handler's output, writes a stderr note, and the dispatcher still exits 0. The exception is a handler that today deliberately exits 2 to block: keep that behaviour, and if one exists, name it in the M1 report.
- **Q4 Codex.** Decided at M3. `codex-hook` stays as the Codex PreToolUse handler for now.
- **Q7.** Claude compaction rows are out of scope here; they belong to #1303.

## M1 pre-work (dispatcher)

1. Add `harnez hook <event>` with a registry: event → ordered handlers. Register every existing handler per the table in the plan. stdin is read once and passed as bytes. The old subcommands keep working as thin aliases until M2 switches `apply`.
2. Implement the merge and isolation above. `handler_timeout` goes in `spec/hooks.yaml` plus a schema, with no Go default.
3. Tests first, at least these:
   - two PreToolUse handlers returning deny and allow → deny, with the deny reason;
   - two `updatedInput` → the first is kept, the second dropped;
   - additionalContext strings joined in order;
   - one handler panics or times out → the other's output is still written, exit 0;
   - handler order is stable;
   - a JSON handler and a text handler on one event are rejected;
   - per event, the dispatcher output for today's real payload shapes equals the old subcommand's output: the golden check that behaviour did not change.
4. Docs: `docs/CLIDesign.md` (hook command surface) and `docs/HarnezComponents.md:28`.
5. Gates: `HTO=0 make test-q1` once, with the log in a file. Commits `feat(hook): … (issue 1304 M1)`, at most 3. Never amend.

## M1 review (host and agy:neon:med reviewer)

M1 delivered (dispatcher): 5f66da33 `internal/hookdispatch` with merge, budget and isolation; b9842066 `harnez hook <event>` registry and goldens. Dev q1: 83 ok, 0 `--- FAIL`.

Clean, confirmed by the reviewer and the host:
- No handler reads `os.Stdin`, writes `os.Stdout`, calls `os.Exit` or exits 2.
- The tool filters match `config.yaml:326-366` and `spec/claude-compaction.yaml` exactly.
- A single reply passes through byte for byte.
- The sound player is detached (`Setsid`), so `cancel()` kills nothing.

Findings (reviewer rev-1304-m1, checked by the host against the diff):
1. **MED** `merge.go:77-88`: a merged reply keeps `updatedInput` next to a winning `deny`, and a JSON `null` `updatedInput` counts as a rewrite.
2. **MED** `merge.go:12-23` and `hookdispatch.go:173-195`: a second handler's `{}` triggers the merge, and the merge drops top-level keys it does not model (`continue`, `stopReason`, `suppressOutput`).
3. **MED** `hookdispatch.go:144-168`: a timed-out handler keeps running and can still write SQLite, telemetry or marker files until the process exits.
4. **MED** `hook_dispatch_test.go:121-189`: some goldens are vacuous:
   - Skill, UserPromptSubmit and PreCompact compare `"" == ""` and check no side effect.
   - Edit compares `{}` with `{}`.
   - PostToolUseFailure, PostCompact and Stop have no case.
5. **LOW**: no test validates `spec/hooks.yaml` against its schema, and the pattern allows `0s`.
6. **LOW** `hook.go:945-978`: `claudeCompactResetInstalled()` greps settings for `harnez hook compact-reset` and will return false after M2.

## Decision (decide mode, M1 review)

- Finding 3 is accepted and documented, not fixed. Reason: the process exits right after `Dispatch` returns, so the late write window is one handler's own state, and no two handlers share state (telemetry vs marker dir). Plumbing a context through six handlers is out of scope. Note it in the `Dispatch` doc comment and in `docs/CLIDesign.md`.

## M2 pre-work (apply: one entry per event)

M1 fixes first:
1. Merge: a winning `deny` clears `updatedInput`. A `null` or empty `updatedInput` is not a rewrite. Add tests for both.
2. Treat a reply that is exactly `{}` (after trimming) as no reply, so one real reply still passes byte for byte. On a real merge, keep top-level keys the merge does not model, first reply wins, with a stderr note on a conflict. Add a test with `continue: false` plus a second handler's reply.
3. Goldens with teeth:
   - Skill and UserPromptSubmit assert the DB row (use a temp DB the test owns).
   - PreCompact asserts that the marker dir is removed and the telemetry call is made, via an injectable seam.
   - Edit uses a stub backend that returns a deny, and the golden compares the deny JSON.
   - Add PostToolUseFailure, PostCompact and Stop cases. Stop uses an injected `Play`.
4. Validate `spec/hooks.yaml` against `spec/schemas/hooks.schema.json` in a test, the way other spec tests do. Make the pattern reject `0` durations.

M2 proper:
5. `apply` writes one Claude entry per event, each `harnez hook <Event>`. PreToolUse gets the matcher `Bash|View|Read|ReadMultipleFiles|Edit|Write`; PostToolUse and PostToolUseFailure get `Skill`. Derive both matchers from the registry's `Tools`, not from a second list. Events without a matcher get no matcher. That makes 8 entries.
   - Sources: `config.yaml:326-366`, `spec/claude-compaction.yaml` and `internal/claude/config.go:330-342`.
6. Migration: `removeHarnezHooks` (`internal/claude/apply_settings.go:117-162`) must also strip an absolute-path harnez command (`/…/harnez …`). Keep every non-harnez entry untouched. Test with a settings fixture that has today's 14 entries, a foreign hook on the same event and an absolute-path entry: it gives exactly 8 harnez entries and keeps the foreign one.
7. Fix `claudeCompactResetInstalled()` (`hook.go:945-978`) for the new command. Check that `status.go:42` derives its expected count from the registry.
8. Update tests: `TestCompactionHooksPreserveForeignAndAreIdempotent`, `TestApplyWithJevEnvironmentIsIdempotent`, `TestApplyDiffRepairCycle` and `status_test.go:120`.
9. Keep the old subcommands working: sessions that have not reloaded still call them.
10. Docs: `docs/CLIDesign.md`, plus `docs/UsageCollection.md:56` if it names the hook commands.
11. Gates: `HTO=0 make test-q1` once, with the log in a file. Commits `feat(hook): … (issue 1304 M2)` / `fix(hook): …`, at most 3. Never amend.

The host runs `make smoke` and a scratch-HOME apply → re-apply → drift-repair run before M2 counts as landed.

## M2 review (host and agy:neon:med reviewer)

M2 delivered (one entry per event): 967b120f fixes the M1 review findings; b0fbda46 makes `apply` write 8 Claude entries from `hookdispatch.ClaudeManifest`. Dev q1: 83 ok, 0 `--- FAIL`.

Host check on the installed binary in a scratch HOME:
- `apply` writes the 8 entries, and a re-apply prints `No changes.`
- `harnez hook PreToolUse` gives the following:

  | Payload | Output |
  |---|---|
  | Bash | the `⚙` rewrite |
  | Read | `{}` |
  | Grep (no handler) | nothing, exit 0 |

- `harnez hook SessionStart` prints the instructions.
- The real `~/.claude/settings.json` is untouched.

Reviewer rev-1304-m2, checked by the host:
1. **MED** `internal/claude/config.go:28,310`: a `-c` config with a `hooks:` key is silently ignored.
2. **MED** `internal/hookdispatch/claude_manifest.go:13-29`: the event/handler/tool wiring moved from `config.yaml`/`spec/` into a Go var. This breaks `docs/Spec.md:15,178-181`.
3. **LOW** `apply_settings.go:114-117`: legacy harnez entries with an `env X=1` prefix or a quoted path are not migrated.
4. **LOW**: the Bash, Read and SessionStart goldens leave `wantOut` empty.
5. **LOW**: stale docs:
   - `docs/HookRewritePattern.md:89,173` and `docs/HarnezComponents.md:98,207,243,344,387` still name the `config.yaml` `hooks:` list or the old commands;
   - `docs/UsageCollection.md:56` names `harnez codex-telemetry`.
6. Dismissed: "the matcher is an unanchored regex, so anchor it". The hooks docs say a matcher made only of letters, digits, `_`, `-`, spaces, `,` and `|` is an exact list, which `Bash|View|Read|ReadMultipleFiles|Edit|Write` is. Anchoring with `^(…)$` would turn it into a regex, so keep it.

## Decision (decide mode, M2 review)

- Move the manifest to `spec/hooks.yaml`, with schema entries. It holds, per agent and event, the handlers in run order with name, kind and tools. Go keeps only the name → run-function binding.
  - At load, every spec handler name must have a Go binding and every Go binding a spec entry; anything else is an error, checked by a test.
  - Reason: this is the repo rule, and M3 adds Codex to the same file.
- A `hooks:` key in a `-c` config is an error that names `spec/hooks.yaml`; it is not silently ignored. Reason: silent loss of a user setting is worse than a clear failure. Custom per-user hooks are not a feature today.

## M3 pre-work (M2 follow-ups, then Codex)

M2 follow-ups:
1. Move `ClaudeManifest` into `spec/hooks.yaml` and its schema, with the binding check above. The entries `apply` writes must stay byte-identical: the M2 migration test and `TestApplyDiffRepairCycle` stay unchanged.
2. `internal/claude/config.go`: make a `hooks:` key in a loaded config an error. Test it with `hooks: []` and with a custom entry.
3. `isHarnezCommand`: also match an `env VAR=… harnez …` prefix and a quoted first word whose base name is `harnez`. Keep `harnez-foo` and wrapper scripts foreign. Use table tests for each case, including the near misses.
4. Bash, Read and SessionStart goldens: set a non-empty `wantOut` or a field assertion. Bash: the `updatedInput.command` starts with `⚙`. Read: `{}` on a plain file. SessionStart: the output contains the instruction rule line.
5. Fix the stale docs listed in finding 5.

M3 proper (Codex, `internal/codex/hooks.go:59-96`):
6. One Codex entry per event, `harnez hook codex <Event>` or the same `harnez hook <Event>` with an agent flag; the developer picks one and names the reason in the report.
   - The manifest goes in `spec/hooks.yaml` under `codex`. `codex-hook` (PreToolUse Bash) and `codex-telemetry` become handlers.
   - The Codex reply shape (`cmd/harnez/codexhooks.go:41-48`) must stay byte-identical; prove it with goldens per event against the old subcommands.
7. Codex migration: `Apply` and `Remove` (`hooks.go:175,328`) replace old harnez entries and keep foreign ones. Test it like the Claude migration test. Old subcommands keep working.
8. Docs: `docs/CodexHooks.md`.
9. Gates: `HTO=0 make test-q1` once, with the log in a file. Commits `fix(hook): … (issue 1304 M3)` / `feat(hook): …`, at most 3. Never amend.
