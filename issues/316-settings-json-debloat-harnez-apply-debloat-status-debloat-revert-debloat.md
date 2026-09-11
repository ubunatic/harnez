# 316 — settings.json debloat: harnez apply --debloat / status --debloat / revert --debloat

**Status**: Draft
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
  command scope" section in this repo's `CLAUDE.md`): `apply` writes
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
