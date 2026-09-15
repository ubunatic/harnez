# 316 — settings.json debloat: harnez apply --debloat / status --debloat / revert --debloat

**Status**: Closed
**Priority**: P1 (High)
**Severity**: Minor
**Category**: Feature

---

## Summary

External feature request: add support to `harnez apply` for generating, merging,
and toggling a "debloat" configuration in `~/.claude/settings.json` that denies a
set of Claude Code tools and disables several global feature toggles, to reduce
system-prompt/context size and tool-definition token cost.

## Target JSON shape (as proposed)

```json
{
  "permissions": {
    "deny": [
      "EnterPlanMode",
      "ExitPlanMode",
      "DesignSync",
      "NotebookEdit",
      "SendMessage",
      "PushNotification",
      "RemoteTrigger",
      "ReportFindings",
      "ScheduleWakeup",
      "AskUserQuestion",
      "CronCreate",
      "CronDelete",
      "CronList"
    ]
  },
  "disableBundledSkills": true,
  "disableWorkflows": true,
  "disableRemoteControl": true,
  "disableClaudeAiConnectors": true,
  "disableArtifact": true
}
```

## Proposed data structures (Go)

```go
type ClaudePermissions struct {
    Deny []string `json:"deny,omitempty" yaml:"deny,omitempty"`
}

type ClaudeSettings struct {
    Permissions               *ClaudePermissions `json:"permissions,omitempty" yaml:"permissions,omitempty"`
    DisableBundledSkills      *bool              `json:"disableBundledSkills,omitempty" yaml:"disable_bundled_skills,omitempty"`
    DisableWorkflows          *bool              `json:"disableWorkflows,omitempty" yaml:"disable_workflows,omitempty"`
    DisableRemoteControl      *bool              `json:"disableRemoteControl,omitempty" yaml:"disable_remote_control,omitempty"`
    DisableClaudeAiConnectors *bool              `json:"disableClaudeAiConnectors,omitempty" yaml:"disable_claude_ai_connectors,omitempty"`
    DisableArtifact           *bool              `json:"disableArtifact,omitempty" yaml:"disable_artifact,omitempty"`
}
```

## Proposed CLI commands

- `harnez apply --debloat [--preset full|minimal]` — merge deny list + disable
  flags into `~/.claude/settings.json` non-destructively; must preserve existing
  custom keys (auth tokens, user-defined aliases). Default preset: "full" (all of
  Section 2 above).
- `harnez status --debloat` — read current config, print a formatted status table
  of denied tools and enabled global toggles.
- `harnez revert --debloat` — remove managed entries; reset boolean toggles to
  their default (false/absent) without corrupting unrelated JSON fields.

## Merge semantics

- **Idempotency**: don't overwrite `permissions.deny` if user-defined entries
  already exist — set-union of strings, not replace.
- **Safety**: back up to `~/.claude/settings.json.bak.<timestamp>` before any write.

## Definition of done (as proposed)

- Unit tests: merge into empty/missing file; merge into a file with existing
  user-defined keys (zero data loss); clean revert.
- Manual verification: applying the "full" preset actually strips the targeted
  tool definitions from a live Claude Code session.
- Code review sign-off.

## Open design questions before implementation

- **`apply` is global-only by design** (see `docs/CLIDesign.md` and the "CLI
  command scope" section in this repo's `AGENTS.md`): `apply` writes
  `~/.claude/settings.json` for the whole user, not per-project. A debloat preset
  applied here affects every project the user works in, including ones that rely
  on the denied tools. Confirm this is the intended scope (vs. a project-local
  `init` flag) before building.
- **The proposed "full" deny list removes interactive/safety-relevant tools**:
  `AskUserQuestion`, `ScheduleWakeup`, `ReportFindings`, `SendMessage`,
  `ExitPlanMode`/`EnterPlanMode`. These aren't pure token-cost tools — denying
  them changes agent behavior (no more clarifying questions, no scheduled
  wakeups, no structured findings reports, no plan-mode gate). Worth confirming
  with the user this is the desired trade-off per preset, and considering a
  narrower default "minimal" preset that targets genuinely unused
  integration-only tools (`DesignSync`, `PushNotification`, `RemoteTrigger`,
  `CronCreate`/`CronDelete`/`CronList`) without touching interaction/safety tools.
- Verify the exact current key names/shape in Claude Code's `settings.json`
  schema (permissions.deny entries, `disableBundledSkills` etc.) against a real
  installed version before writing the Go structs — the request doesn't cite a
  version or source for this schema.

## External design review (Codex/Astra advisor, 2026-09-11)

Ran via `harnez-advisor` (issue 303): `codex exec -m gpt-6-astra -c
model_reasoning_effort=low`, read-only — no project files were changed.

**Verdict: No-go as written; conditional go after these revisions.**

1. **Deny list / preset**: Don't make `full` the default. `AskUserQuestion`,
   `EnterPlanMode`, `ExitPlanMode`, `SendMessage`, `ReportFindings`,
   `ScheduleWakeup` support interaction, approval, and coordination — their
   removal is a behavioral choice, not token-cost cleanup, and needs explicit
   opt-in. Defer `full` from v1 or rename it `aggressive` and require explicit
   selection. A `minimal` preset should be limited to verified
   integration-only tools (`DesignSync`, `PushNotification`, `RemoteTrigger`);
   keep `NotebookEdit` and the `Cron*` flags separately selectable rather than
   bundled in.
2. **Scope**: `apply`'s global scope is architecturally coherent for a
   genuine user-wide preference (does not itself violate `CLIDesign.md`), but
   any project-dependent opt-out belongs in project-local settings, not by
   making `apply` project-aware. Preview/status output should explicitly name
   the target file and state "affects every project for this user."
3. **Structs**: The proposed `*bool` struct model is acceptable as a patch
   description but insufficient as a full settings-file model — round-
   tripping the whole file through it would silently drop unknown fields
   (`permissions.allow`, `ask`, etc.). Require a raw-JSON-preserving merge
   that only appends missing exact strings, and reconcile with the
   `Permissions{Allow, Deny}` type already in `internal/claude/config.go:100`
   rather than introducing a competing model.
4. **Revert semantics (blocker)**: "reset booleans to false" is wrong —
   revert must restore prior state, not defaults. Requires a persisted
   ownership record (what harnez actually added vs. what pre-existed) so
   revert only undoes owned changes and never clobbers a pre-existing `true`
   or a pre-existing deny entry.
5. **Acceptance criteria**: add a verified Claude Code settings-schema
   version/source; a canary proving actual tool-definition removal and
   measured context reduction (not just a successful JSON write); behavioral
   tests that clarification/plan-approval/coordination still work under the
   default preset; and tests for pre-existing true/false/absent values,
   overlapping denies, unknown nested fields, and repeated apply/revert
   cycles.

**Disposition**: implementation should not start until the preset, ownership
model, and validation gate above are captured in this ticket. A narrow,
explicit-opt-in global preset remains defensible; the proposed `full`-by-
default and reset-to-default revert are not.

## Decided design (2026-09-15)

Resolved with the user against the two open blockers above.

1. **Presets**:
   - `--preset minimal` (default when `--debloat` given with no `--preset`):
     denies only verified integration-only tools — `DesignSync`,
     `PushNotification`, `RemoteTrigger`.
   - `--preset aggressive`: everything in `minimal`, plus the
     interaction/safety-relevant tools from the original "full" list —
     `AskUserQuestion`, `ScheduleWakeup`, `ReportFindings`, `SendMessage`,
     `EnterPlanMode`, `ExitPlanMode`. Only applied when the user explicitly
     passes `--preset aggressive` — never the default.
   - `NotebookEdit` and `CronCreate`/`CronDelete`/`CronList` stay separately
     selectable (e.g. `--debloat-notebook-edit`, `--debloat-cron`), not bundled
     into either preset by default.
   - The `disableBundledSkills`/`disableWorkflows`/`disableRemoteControl`/
     `disableClaudeAiConnectors`/`disableArtifact` booleans stay individually
     selectable flags, off by default, independent of preset choice.

2. **Ownership/revert model**: persist a sidecar ownership record (e.g.
   `~/.claude/.harnez-debloat.json`) written at apply time, capturing the
   **prior value** of every key/entry harnez is about to touch (each deny
   entry it adds; each boolean's prior value, including "absent"). `harnez
   revert --debloat` reads this record and restores exactly those prior
   values (re-adds a pre-existing deny entry if harnez's own entry happened
   to duplicate one already present; restores a pre-existing `true` instead
   of resetting to `false`; removes a boolean key entirely if it was absent
   before), then deletes the record. This avoids clobbering user state and
   satisfies review point 4.

3. Still required per the review before this ships: reconcile with the
   existing raw-JSON-preserving merge (`applySettingsJSON` /
   `managedSettingsKeys` in `internal/claude/apply.go`) rather than
   introducing a competing `*bool` struct model that round-trips the whole
   file — new debloat keys should be merged the same way, with unknown
   nested fields preserved untouched. `status --debloat` should name the
   target file explicitly and state it affects every project for the user.

## Implementation (2026-09-15)

Shipped in `internal/claude/debloat.go` (+ `cmd/harnez/main.go` wiring):

- `harnez apply --debloat[=<any>] [--debloat-preset minimal|aggressive]
  [--debloat-notebook-edit] [--debloat-cron] [--debloat-disable-*]` — merges
  into `<target>/settings.json` via `jsonc.Read`/`jsonc.UnionStrings`/
  `jsonc.MarshalPretty`, the same raw-map approach `applySettingsJSON` uses,
  so unrelated fields (`permissions.allow`, `model`, unknown keys) round-trip
  untouched. `--debloat` alone defaults to `minimal`; `aggressive` requires
  the explicit `--debloat-preset aggressive` flag.
- `harnez status --debloat` — prints every known debloat-managed deny entry
  and toggle with on/off + harnez-managed/pre-existing state, naming the
  target settings.json path and the "affects every project" caveat.
- `harnez revert --debloat` (new top-level command) — restores prior state
  from a sidecar ownership record at `<target>/.harnez-debloat.json`, which
  captures each touched key's *pre-debloat* value only the first time
  harnez touches it (so stacking minimal → aggressive → revert restores the
  true original state, not just the last apply's state), then deletes the
  record.
- Tests in `internal/claude/debloat_test.go` cover: minimal-on-empty, zero
  data loss with pre-existing unrelated fields/deny entries, revert leaving
  pre-existing deny entries alone, revert restoring a prior `true` toggle,
  revert removing a toggle that was absent before, a full apply→revert
  round trip (JSON-semantic diff), aggressive never triggered by `--debloat`
  alone, revert with no record erroring, and status running with/without a
  record. `go build ./...` and `go test ./...` pass with no regressions.
- Verified live end-to-end with the built binary against a temp target dir
  (apply → status → revert), confirming actual settings.json content at
  each step.
- Measured actual context-token impact per preset with `claude -p "/context"`
  in a clean test project: `minimal` ~0% (only trims deferred-tool schemas),
  `aggressive` ~7% (2.5k tokens) — see
  `docs/studies/2026-09-15-debloat-context-usage-measurement-and-cli-flag-comparison.md`
  for the full table and comparison against `--bare`/`--safe-mode`.

## Follow-up 2 (2026-09-15): `EnterWorktree`/`ExitWorktree`/Google Drive MCP tools added to `aggressive`

Assessed every deferred tool not already covered by a preset against this
project's actual conventions:

- `EnterWorktree`/`ExitWorktree`: this project's own `AGENTS.md` explicitly
  forbids worktree-isolated subagents for harnez work — verified-unused by
  written convention, not just observation. Added to `aggressive_extra_deny`
  (grouped there per the user's call, rather than the more conservative
  `minimal` placement originally proposed).
- `mcp__claude_ai_Google_Drive__authenticate` /
  `complete_authentication`: never used; already covered functionally by
  `disable_claude_ai_connectors`, added to the deny list as well for
  `status --debloat` / `revert --debloat` visibility.
- Considered and explicitly declined: `TaskOutput`/`TaskStop` (required by
  the Zero Zombie Guarantee hygiene phase), `Monitor` (used for background
  event streaming), `EndConversation` (proposed but not requested — left
  out), `WebFetch`/`WebSearch` (ambiguous value, held for a separate
  decision).

Verified live against the real `~/.claude/settings.json`
(revert → reapply → `status --debloat` confirms all four new entries
`on (harnez-managed)`).

## Follow-up (2026-09-15): `SendMessage` removed from `aggressive`, preset content moved to config.yaml

Applying `aggressive` live against the real `~/.claude/settings.json` (with
explicit user go-ahead) showed the deny list denying `SendMessage` cuts off
the ability to send a follow-up message to an already-spawned subagent —
the `Agent` tool alone can spawn and receive a completion handback, but
cannot continue a conversation with it. Since `aggressive` is meant to trim
integration-tool cost, not remove a core agent-coordination capability,
`SendMessage` was dropped from `aggressive_extra_deny`.

Also: the preset deny lists (`minimal_deny`, `aggressive_extra_deny`,
`cron_deny`, `notebook_deny`) were originally hardcoded as Go package vars
in `internal/claude/debloat.go`, which violates this repo's `docs/Spec.md`
rule ("YAML spec files/config.yaml are the single source of truth;
application code must not duplicate or shadow spec values" — config.yaml
already plays this role for `permissions`/`hooks`/etc.). Moved to a new
`debloat:` section in `config.yaml`, loaded via a `DebloatConfig` struct on
`Config`, threaded through `ApplyDebloat`/`StatusDebloat`. `RevertDebloat`
needed no change since it only reads its own sidecar ownership record, not
preset content. Tests updated to load the real embedded config rather than
duplicating literal lists in test code.

## Follow-up (2026-09-16): config-driven user-skill overrides

Following issue 349's live proof, debloat presets now merge `skillOverrides`
from `config.yaml`. Minimal retains 19 slash-oriented workflows as
`user-invocable-only`; aggressive additionally applies that mode to
`domain-modeling`, `evergreen`, and `lmcoder`. The sidecar records each prior
value so `revert --debloat` restores user state exactly, while
`status --debloat` displays active ownership. The retired `fresh-sprint` and
`fresh-sprinter` installed artifacts are removed during `apply` in favour of
the `lean-*` workflow. Live aggressive `/context` verification measured 343
Skills tokens and a second application was idempotent.
