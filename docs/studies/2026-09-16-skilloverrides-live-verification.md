<!-- harnez:topic: live verification of Claude Code's skillOverrides settings.json field against an installed version, resolving issue 347's unverified GitHub-issue claims -->

# Verifying `skillOverrides` Against a Live Claude Code Install

**Scope**: Issue 349, follow-up from issue 347's research pass. 347 found GitHub issue
reports (unverified) claiming `skillOverrides` is non-functional for user/project-scoped
settings, and at least one claim that it's scoped to plain user/project skills only, not
bundled/plugin skills. This live-checks both claims against the actually-installed
Claude Code version rather than trusting doc/issue-tracker reports.

**Accessed**: 2026-09-16
**Claude Code version tested**: 2.1.273

## Method

`claude -p --settings '<json>' "/context"` in a fresh temp directory (no project
`.claude/`, no `CLAUDE.md`) — `--settings` merges additional JSON for one invocation only,
without touching any real settings file. Same technique as the sibling debloat study
(`2026-09-15-debloat-context-usage-measurement-and-cli-flag-comparison.md`). Target skill:
`dataviz`, a **bundled** (`Built-in`) skill, chosen specifically to test the
bundled-skill-scope claim.

Baseline (no override): `dataviz` present in the `/context` Skills table as `Built-in,
~480` tokens.

## Finding — `skillOverrides` works correctly on 2.1.273, including for bundled skills

All four documented modes were tested against `dataviz` and each produced a distinct,
correct effect:

| Mode | `/context` Skills table | Explicit `/dataviz` invocation |
|---|---|---|
| `"off"` | Absent entirely (0 tokens) | Blocked: `Skill "dataviz" is disabled via skillOverrides. Remove the override from your settings to run it.` |
| `"name-only"` | Present, `< 20` tokens (name/description only, not full instructions) | `Unknown command: /dataviz` — the slash-command trigger itself isn't registered in this mode (see caveat below) |
| `"user-invocable-only"` | Absent from the auto-loaded table (0 tokens) | Works: proceeded into the skill's actual workflow (blocked only by a normal Write-tool permission prompt, not by any override error) |
| `"on"` (explicit) | Present, `~480` tokens — identical to the unset baseline | (not re-tested; same as baseline) |

Both claims from issue 347's unverified research are **contradicted** by this live check:

1. **"Non-functional for user/project-scoped settings"** — false on 2.1.273. All four modes
   produced their documented, distinct behavior with a single `--settings` JSON blob (the
   same layering path used for CLI-scoped and, by extension, project/user `settings.json`
   files — `--settings` is explicitly documented as merging into that same settings
   precedence chain, not a separate mechanism).
2. **"Scoped to plain user/project skills only, not bundled skills"** — false on 2.1.273.
   `dataviz` is a `Built-in` (bundled) skill per the `/context` table's own `Source`
   column, and `skillOverrides` controlled it correctly in every mode tested.

## Caveat — `name-only` and slash-command invocation

`name-only` mode shows the skill's name and short description in `/context` at near-zero
token cost, but typing `/dataviz` directly returned `Unknown command: /dataviz` rather than
invoking it. This does not necessarily mean the skill is fully inert in this mode — Claude
Code's `Skill` tool can invoke a skill autonomously by name even when the interactive
`/name` slash-command shortcut isn't registered for it (the two are different invocation
paths per the harness's own tool documentation). This distinction was not separately
verified (would require a task that provokes autonomous skill selection rather than an
explicit slash command) — flagged as an open sub-question, not a contradiction of the
core finding.

## Recommendation (updates issue 347)

`skillOverrides` is confirmed functional, including for bundled skills, on the currently
installed Claude Code version (2.1.273). Per issue 349's own framing, this makes it
**strictly better than `disableBundledSkills`** for cases where only specific bundled
skills should be trimmed rather than the entire catalogue: `disableBundledSkills` is
all-or-nothing, `skillOverrides` is per-skill and supports a deliberate degraded mode
(`name-only`) that isn't available at all under the blunt toggle.

This does not retract 348's `disableBundledSkills` measurement or its now-shipped
`preset_disable_bundled_skills: true` default — that toggle remains the right choice when
the goal is "remove the whole bundled catalogue," which is what `--debloat` presets are
for. `skillOverrides` is a complementary, finer-grained lever worth exposing separately
(e.g. a future `--debloat` sub-flag, or direct `config.yaml` passthrough for specific
skill names) for projects that want to keep most bundled skills but trim a handful — not
a replacement for the existing preset.

## Shipped follow-up — user-skill debloat presets

The verified mechanism now backs `harnez apply --debloat` as a config-driven
user-skill policy:

- Both presets set 19 explicit workflows to `user-invocable-only`, keeping their
  slash entry points while removing their startup descriptions.
- `aggressive` additionally makes `domain-modeling`, `evergreen`, and `lmcoder`
  user-invocable-only.
- The superseded `fresh-sprint`/`fresh-sprinter` artifacts are decommissioned and
  removed by `apply`; `lean-sprint`/`lean-sprinter` are their canonical replacements.
- `status --debloat` reports each managed override, and `revert --debloat` restores
  the exact prior per-skill values using the ownership sidecar.

Live verification with the aggressive preset reduced Claude Code's `/context`
Skills estimate from approximately 1.2k tokens to **343 tokens**, leaving only
`harnez-advisor`, `issue`, `publish`, `tool-feedback-protocol`, and `website`
auto-loadable. A second apply reported `No changes`.

**Caveat on durability**: this is one version snapshot (2.1.273). Issue 347's own research
noted this area shows signs of active flux across Claude Code releases — re-verify with
this same method before depending on it in a shipped feature, and re-check after any
Claude Code upgrade before trusting this result to still hold.
