# 1061 — Investigate skills-only apply removing unrelated managed configuration

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 490, 491, 492, 1052

---

## Problem

During Pi background-task work, `harnez apply --components skills` listed managed Codex/Gemini hook configuration and two distill extensions as removed, although the intended operation was limited to skills. The owner expects `harnez apply` to be safe.

Refinement on 2026-10-31 (commit `99cbc20b`) reproduced this in a throwaway home directory. The removal is real and matches the design in `docs/HarnezComponents.md` §8.4, so the code does what the design says; the design is the problem. `--components X` means "the machine should have only X", while its help text ("component(s) to install") and a documented recipe read as "install or update X".

## Goal

/goal Make `harnez apply --components X` leave everything outside X untouched, make distill adapter removal ownership-aware, make apply and diff report removals truthfully, and verify unrelated configuration is preserved. Stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with the resume point.

Recheck live code before starting. Do not run experiments against the user's live configuration.

## Findings

### Reproduction

Build a throwaway binary and run it with `env -i HOME=<tmp> PATH=<tmp>/bin:/usr/bin:/bin`, working directory `<tmp>/home`. Seed `.claude/settings.json`, `.codex/config.toml` and `.gemini/config/hooks.json` with one user-authored hook each, and create `.gemini`, `.pi/agent/extensions` and `.config/opencode/plugins`.

1. `harnez apply`, then snapshot checksums.
2. `harnez diff --components skills`.
3. `harnez apply --components skills`, then compare checksums.
4. Repeat step 3, then run plain `harnez apply` and compare with step 1.
5. Put a user-authored file at `.pi/agent/extensions/harnez-distill.ts`, then repeat step 3.
6. Put the same file there again, then run plain `harnez apply`.

| Scenario | Observed result |
|---|---|
| Skills-only apply after full apply | 15 changes: all harnez hooks in Claude `settings.json`, Codex and AGY; Claude `statusLine`; both distill adapters; `tool-feedback-protocol` in 5 skill targets |
| User-authored hooks and settings keys | Preserved in all three files |
| Repeat skills-only apply | `No changes.` |
| Plain apply afterwards | Restores files byte-identically (15 changes), but prints the Codex hook-trust review note |
| User file at the Pi adapter path, skills-only apply | **Deleted without warning** |
| User file at the Pi adapter path, plain apply | **Overwritten without warning** |
| `diff --components skills` before removal | Shows only the `settings.json` diff; names no adapter, Codex/AGY hook or skill removal |
| `apply --components usage` (recipe in `docs/AgentStatusLines.md:117`) | 22 changes: the same removals plus the whole Pi background-task runner |
| `apply --components telemetry-only` | Reverts the Pi background-task runner (documented in `docs/PiAgent.md:304`) |

### Root causes

1. **A non-persistent flag has desired-state semantics.** `Resolve` (`internal/claude/components.go:125`) gives a flag and a persisted `components:` key the same meaning. Each phase then removes when its component is off: `applyCodexHooks`, `applyAgyHooks` and `applyDistillAdapters` (`internal/claude/apply_phases.go:216`, `:257`, `:290`), `buildSettingsDoc` (`internal/claude/apply_settings.go:190`, `:222`, `:238`), `skillDisabled` (`internal/claude/apply.go:339`) and `applyPiBackgroundTasks` (`internal/claude/pi_background.go:545`). §8.5 already notes that the flag does not persist, so one scoped run strips the machine until the next plain apply.
2. **Adapter handling ignores ownership.** `applyDistillAdapters` calls `os.Remove` and `writeFileIfChanged` on a configured path without checking who wrote the file (`internal/claude/apply_phases.go:297`, `:309`). Every other removal path checks ownership. This is a real data-loss bug under any semantics.
3. **Reports overstate and previews understate.** Apply prints `removed ~/.codex/config.toml` and `removed ~/.gemini/config/hooks.json` when it only removed harnez hooks from those files (`internal/claude/apply_phases.go:249`, `:281`). `DiffAll` only sets a flag for Codex/AGY and has no adapter check at all (`internal/claude/apply.go:810-834`), so preview cannot show what apply will delete.
4. **The `telemetry` label removes more than telemetry.** `isHarnezCommand` covers every configured hook, including the guards `harnez hook pre-edit`, `harnez hook read`, `harnez hook claude-instructions` and `harnez hook sound`. §8.4 still claims the Stop sound hook "survives every selection", which is stale.
5. **Status lines are asymmetric.** Without `usage`, Claude's status line is removed, but Codex and AGY status lines are neither applied nor removed (`cmd/harnez/apply_cmd.go:86`, `:125`).

### Live machine state (read-only check, 2026-10-31)

`~/.claude/settings.json` has no `hooks` key and no `statusLine`. `~/.codex/config.toml` has no `harnez` hook commands. `~/.gemini/config/hooks.json`, both distill adapters and `tool-feedback-protocol` in the Claude, Codex and Gemini skill targets are absent. `settings.json`, `config.toml`, `~/.gemini/config` and managed skills all carry the modification time 2026-10-30 17:23.

This matches a skills-only apply that was never followed by a plain apply, although the timestamp alone does not prove which command ran. It means the owner's hooks and guards have probably been off since then. Whether a user-authored adapter was lost cannot be established; this machine has no surviving copy to compare. Recovery is M0.

## Decisions

**D1, confirmed by the owner (2026-11-01).** The `--components` flag becomes scoped, and a persisted `components:` key stays desired-state.

- `apply --components X` reconciles only X and leaves unselected components untouched, with no install and no removal. `diff` and `status` use the same scope, so unselected parts are not drift.
- `apply --components X --prune` gives today's removal for one run. Check `docs/CLIDesign.md` for shorthand conflicts first.
- A `components:` key in the loaded config keeps removing unselected components, because it is a persistent, explicit choice. A flag overrides its scope, and without `--prune` the flag run removes nothing.
- Implementation: the scope travels inside the resolved `Set` (`claude.Scope`, `Set.Scoped`), not in a separate `ApplyOptions` field, so every existing selection parameter carries it. `--prune` is long-only (`-d` is taken, see `docs/CLIDesign.md`). `status` takes no `--prune`: it is read-only and removes nothing.

Rejected: keeping the removal and only adding a warning, because the documented recipe and the natural reading of the flag would both stay destructive.

**D2, decided:** the always-on writes in §8.3 (`model`, `effortLevel`, `permissions`, `env`, `spinnerVerbs`, decommissioned-artifact cleanup, shims) stay as they are. They are additive and already documented. Whether guards deserve their own component is out of scope here; this ticket only corrects the docs.

## Milestones

- **M0 — Live recovery (owner action, no code).** Owner runs `harnez diff`, then `harnez apply`. Codex prints a hook-trust review note afterwards. Do not wait for M3; running only plain `harnez apply` is safe from the removal path.
- **M1 — Adapter ownership (no decision needed).** Treat a regular file as harnez-owned only if it starts with `// Managed by harnez.`, which also covers older generated versions. A missing file installs normally. Any other file, or a symlink, is kept and reported as `kept <path>: not harnez-managed`, on both the install and the remove path, and apply still succeeds.
- **M2 — Truthful reporting (no decision needed).** Print `removed harnez hooks from <path>` for Codex and AGY. End a run with a summary naming each removal and its reason, for example `telemetry not selected`. `diff` names every adapter, Codex/AGY hook and skill that apply would remove or refuse, and `diff --exit-code` fails when only adapters would change.
- **M3 — Scoped flag (requires D1).** `Resolve` returns the selection plus whether it is scoped, and `ApplyOptions` carries that (`internal/claude/operation_options.go`). Under scope, `buildSettingsDoc` omits the `hooks` and `statusLine` keys of unselected components, which `applyMerge` then leaves alone. Hook, adapter and Pi phases skip instead of removing, and `skillDisabled` skips skills whose unmet `requires:` is only out of scope.
- **M4 — Status-line parity.** In desired-state mode without `usage`, remove the Codex and AGY status lines too, using the existing `RemoveStatusLine` functions, and report it in `diff`.
- **M5 — Docs.** Update `--components` help text, `docs/HarnezComponents.md` §8.3–§8.5 and §8.7–§8.8 (including the stale sound-hook claim and the guards), `docs/PiAgent.md:304-306` (revert now needs `--prune` or a config key), and `docs/AgentStatusLines.md:117`. Add the Pi runner to §8.3 and §8.7.

## Acceptance Criteria

- [ ] After a full apply, `apply --components skills` leaves these byte-identical: Claude `hooks` and `statusLine`, Codex and AGY hooks, both distill adapters, `tool-feedback-protocol`, and the Pi runner stays installed.
- [ ] The same holds for `--components usage` and `--components docs` with respect to their unselected components, and `diff --components <same> --exit-code` then reports no drift for unselected components.
- [ ] `apply --components skills --prune` and a `components: [skills]` config both remove what they remove today, and only harnez-owned entries.
- [ ] A user-authored file at either adapter path survives install, remove and `--prune`, and is reported as kept.
- [ ] User-authored hooks and unknown keys survive every selection in `settings.json`, `config.toml` and `hooks.json`.
- [ ] Repeating any selection reports `No changes.`, and plain apply after a pruned run restores the full state byte-identically.
- [ ] `diff` lists every path apply would remove before apply runs, and apply no longer claims to remove a file it only edited.
- [ ] Docs and help text describe the behaviour verified above.

## Test Plan

- `internal/claude/distill_adapters_test.go`: managed file updated and removed; user file and symlink kept on both paths; older managed header recognised.
- `internal/claude/components_test.go`: table over `skills`, `usage`, `docs` and `telemetry-only` in scoped mode, comparing checksums of unselected paths; keep the existing removal tests by running them with prune or a config key.
- `internal/claude/requires_selection_test.go`: a skill with an unmet `requires:` stays installed under scope and is removed under prune.
- `cmd/harnez/components_e2e_test.go`: extend the CLI round trip with Codex, AGY, adapters and the Pi runner; assert `--prune`, `diff --exit-code`, and the absence of `removed <config.toml>` wording. `TestApplyDiffCLI_ComponentsDocsOnlyRoundTrip` and `cmd/harnez/apply_test.go:127` pin today's flag removal and must move to `--prune` deliberately, not be weakened.
- All tests use temporary homes. Finish with `make test-q1` and `make install`; the implementation is a larger change, so it needs an independent review pass before commit.
