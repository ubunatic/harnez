# 149 — Agent-specific instruction profiles, first use case: Codex async-wait guidance

**Status**: Closed — mechanism + Codex content shipped, reviewed, and committed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[144-codex-subagent-model-selection-policy]] (same class of problem — a Codex-only
correction with no home in the shared instruction set — likely becomes the second use case for
this same mechanism), [[151-on-demand-doc-lookup-vs-materialized-instructions-research]] (research
ticket asking whether this profile mechanism should also apply per-repository and temporarily, and
whether materialization itself is the right delivery model for generic docs — depends on this
ticket's mechanism existing first), [[130-instruction-distribution-audit-followups]] (prior distribution audit,
scoped to *uniform* content delivered to different targets, not *differentiated* per-agent
content), [[108-subagent-dispatch-sequential-default-and-issue-number-race-guard]], `config.yaml`
`agents_md` section

## Problem

harnez's `agents_md` distribution (`config.yaml`) currently sends the same instruction content to
every harness target — `~/.claude/CLAUDE.md`, `AGENTS.md` (read by Codex, Gemini, Prime Agent via
symlink/convention). There is no mechanism for a rule, skill, or command to apply to *one* agent
only. Every instruction added to the shared sections is either global noise for agents it doesn't
apply to, or has nowhere to live at all.

A concrete case surfaced this: Codex subagents lack Claude Code's smooth async subagent-handoff
flow. In Claude Code, dispatching a subagent lets the host stay responsive; when the subagent
finishes, the host is notified and can take over (or chain the next dispatch) without the host
polling for completion. Codex has no equivalent notification path for background jobs/subagents —
self-assessment showed Codex instead falls back to writing its own polling loop to wait on a
background task, which is exactly the anti-pattern `docs/practices/AgenticLoop.md`'s Zero Zombie
Guarantee and issue [[055]] (no long sleep, use scheduled wakeups) already warn against for other
contexts.

The needed correction — "do not poll for background job completion; use \<Codex's actual async
primitive\>" — is Codex-specific. Codex doesn't have the missing capability being warned about, so
telling Claude Code or agy "don't poll" would be inert advice that only adds noise to their already
correct behavior. This is the first concrete case where a *shared* instruction set is the wrong
shape, motivating a real per-agent profile mechanism rather than another shared-section bullet.

## Scope

1. **Design an agent-specific profile mechanism** in harnez's instruction-distribution system
   (`config.yaml`/`apply.go`), parallel to the existing shared `agents_md.global`/project sections,
   that can carry rules, skills, or commands scoped to exactly one target agent (Claude Code, agy,
   Codex, Gemini, Prime Agent) without appearing in the others' generated files.
2. **First concrete content**: a Codex-only instruction correcting the background-job/subagent-wait
   behavior — identify Codex's actual supported async-wait or notification primitive (if any) and
   instruct it to use that instead of a self-written polling loop; if Codex genuinely has no
   equivalent to Claude Code's subagent-completion notification, say so explicitly and give the
   least-bad fallback (e.g., a bounded/backoff poll rather than a tight loop) rather than leaving
   the gap unaddressed.
3. **Keep this out of the shared sections.** Do not add the correction to `agents_md.global` or any
   section that reaches Claude Code/agy — verify after implementation that neither's generated
   instructions changed.
4. Document the new mechanism in `docs/CLIDesign.md` or `docs/other/Spec.md` (whichever is the
   better fit) so future agent-specific corrections (starting with [[144]]) have a defined home
   instead of being bolted onto the shared sections out of convenience.

## Out of Scope

- [[144]]'s actual model-selection policy content — this ticket only establishes the mechanism;
  144 can migrate into it as a second use case once the mechanism exists, but is not blocked on
  this ticket and should not be implemented here.
- Building a general templating/inheritance system across all five agent targets — start with the
  minimum needed to scope one instruction to one agent; generalize later if a third use case shows
  the pattern repeating.

---

## Implementation Plan

### Key structural finding (research, 2026-09-04)

There is currently **no per-agent instruction file at all**. `apply.go:606-650` applies
`agents_md.global.sections` to a `ruleTargets` list that is exactly:

1. `fsutil.ExpandHome(cfg.AgentsMD.Global.Target)` → `~/.claude/CLAUDE.md`
2. `primeAgentRoot(cfg)/AGENTS.md` → `~/.prime/agent/AGENTS.md` (only if the root exists)

and then symlinks `~/AGENTS.md → ~/.claude/CLAUDE.md` (`config.yaml:301-302`, verified on disk).
Codex and Gemini read that same symlinked file, so *today the Claude file and the Codex file are
literally the same bytes*. Per-agent scoping therefore cannot be done by filtering sections into
existing targets — it needs a **new, agent-owned target file** that only that agent reads. Codex
already has a config root (`~/.codex/`, with a `rules/` dir and `config.toml`) and harnez already
addresses it (`codex_skills_target`, `codex_hooks_target`), so the plumbing precedent exists.

### Steps

1. **Config schema** (`internal/claude/config.go`): add to `AgentsMD`:
   ```go
   Agents map[string]AgentsMDTarget `yaml:"agents"`
   ```
   Reuse `AgentsMDTarget` verbatim (it already carries `Target`, `Symlink`, `Sections`) rather
   than inventing a new struct. Map key = agent id (`codex`, `claude`, `agy`, `gemini`, `prime`),
   matching the `AgentID` vocabulary `internal/usage` already uses.

2. **Apply pass** (`internal/claude/apply.go`): add an `agents_md.agents` loop directly after the
   existing `Global` block (~line 650). It is a near-copy of the global loop: for each entry,
   expand `Target`, run `applySectionMD(target, s.Name, s.Content)` per section, honour the same
   `s.RateFeedback && disableRateFeedback` → `markdown.Clean` removal path, then `printResult` /
   `addStat`. Skip an entry whose parent dir does not exist (same posture as `primeAgentRoot`
   returning "" — do not create `~/.codex` for a user who does not run Codex).

3. **Config content** (`config.yaml`, after the `agents_md.global` block): add
   ```yaml
   agents:
     codex:
       target: ~/.codex/AGENTS.md
       sections:
         - name: Background Job Waiting
           content: |
             ...
   ```
   Content per scope item 2: state that Codex has no host-notified subagent-completion callback,
   so a host must not spin a tight poll loop; prescribe the least-bad supported primitive
   (`collaboration.spawn_agent` + a bounded/backoff check, or an explicit user-visible handoff —
   see [[156]] for what the Agents view actually shows). Verify the exact tool names against a
   live Codex environment before wording this as fact; do not invent a primitive.

4. **`clean` / `diff` / `status` parity** — the three commands that enumerate managed sections
   must learn about the new map or they will report drift and fail to clean:
   - `apply.go:848` (diff block) and `apply.go:922` (clean block): mirror the same loop.
   - `internal/claude/status.go:70-84`: add agent targets to `ruleTargets` and the section census
     (`status.go:42-43` prints `N global, N local` — extend to include agent counts).

5. **Tests** (`internal/claude/apply_test.go` or a new `agents_profile_test.go`): assert with a
   temp HOME that (a) an `agents.codex` section lands in `~/.codex/AGENTS.md`, (b) it does **not**
   appear in `~/.claude/CLAUDE.md` or `~/.prime/agent/AGENTS.md` — this is scope item 3's
   verification, and it must be a real negative assertion, not just a positive one, (c) apply is
   idempotent on a second run, (d) `clean` removes the agent section.

6. **Docs**: `docs/CLIDesign.md` is the right home (it already owns the apply-vs-init target
   table). Add a short subsection under the command-responsibilities table describing
   `agents_md.agents` as the place for agent-differentiated content, with the rule: *shared
   behaviour goes in `global`; a correction that would be inert or wrong for another agent goes in
   `agents.<id>`*. Also add the new file to `docs/CLIDesign.md`'s target listing.

### Design decisions / tradeoffs

- **New file per agent, not filtered sections in a shared file.** The symlink makes filtering
  impossible without breaking `~/AGENTS.md → ~/.claude/CLAUDE.md`, and breaking that symlink is a
  much larger behavioural change than adding one small extra file.
- **Reuse `AgentsMDTarget`, not a new `AgentProfile` type.** Scope explicitly forbids building a
  templating/inheritance system; a map of the existing struct is the minimum that works.
- **Map, not a repeated list with an `agent:` field** — makes "one profile per agent" structurally
  enforced and gives config authors an obvious lookup key.
- **Additive only**: nothing about `global`/`local` changes, so an unset `agents:` key is a no-op.

### Risks / open questions

- **Does Codex actually read `~/.codex/AGENTS.md`? — VERIFIED YES (2026-09-07).** Wrote a unique
  marker line to `~/.codex/AGENTS.md` (which did not previously exist on this machine) and ran a
  fresh headless `codex exec "quote the secret marker string"` from `/home/uwe/projects/harnez`.
  Codex correctly quoted the marker with no other exposure to it, confirming it reads
  `~/.codex/AGENTS.md` as a real instruction anchor. Test file removed after verification. The
  originally planned anchor (`~/.codex/AGENTS.md`) is correct — no fallback (e.g.
  `~/.codex/rules/harnez.rules`, which is a structured permission-rule file, not prose) is needed.
- Content accuracy for the Codex async primitive is unverified; the mechanism can land before the
  content is final, but do not ship guessed tool names.
- Adding a fourth managed instruction file increases the surface `status`/`diff`/`clean` must keep
  in sync — step 4 is not optional cleanup, it is part of the feature.

### Scope estimate

**Medium** — the mechanism itself is small (one struct field, one apply loop, three parity
updates, tests), but the parity work across apply/diff/clean/status plus the empirical Codex
verification is what pushes it past "small".

---

## Implementation Record (2026-09-07)

Implemented as planned, with two deviations noted below.

**Files changed:**
- `internal/claude/config.go` — added `Agents map[string]AgentsMDTarget` to `AgentsMD`.
- `internal/claude/apply.go` — added a `sortedAgentIDs` helper (deterministic map iteration,
  not called out in the plan but needed since Go map order is randomized) and mirrored
  apply/diff/clean loops over `cfg.AgentsMD.Agents`, each soft-skipping an entry whose
  target's parent directory doesn't exist on disk.
- `internal/claude/status.go` — extended the `agents_md:` census line to `%d global, %d
  local, %d agent` and added agent targets to the `ruleTargets` check census.
- `config.yaml` — added `agents_md.agents.codex` with a `Background Job Waiting` section.
- `internal/claude/agents_profile_test.go` (new) — 7 tests: lands-in-own-target,
  does-not-leak-to-global/prime (negative assertion), soft-skip-missing-root, idempotent
  apply, clean removes it, no false diff-drift after apply, status census count.
- `docs/CLIDesign.md` — new "Agent-specific instruction profiles (`agents_md.agents`)"
  section with the global-vs-agents rule of thumb and a target-file table; added the new
  apply-flow line.
- **Deviation (test hygiene fix, not in original plan)**: `claudeskills_test.go`,
  `issue_skill_test.go`, `telemetry_hook_test.go`, `toolfeedback_disable_test.go`,
  `toolfeedback_test.go`, `integration_test.go` — each adds `cfg.AgentsMD.Agents = nil`
  next to the existing `cfg.AgentsMD.Global.Symlink = ""` override. These tests call
  `LoadConfigEmbedded()` (the real `config.yaml`) and run `ApplyAll`/`CleanAll` without
  overriding `$HOME`. Once `config.yaml` declared a real `~/.codex/AGENTS.md` default
  target, `TestIntegrationWorkflow` on this dev machine (which has `~/.codex` since Codex
  is installed) started writing to and then cleaning the real file on every `go test`
  run — the exact real-`$HOME` test-pollution failure mode the surrounding tests already
  guard against for `CodexSkillsTarget`/`ClaudeSkillsTarget`/etc. Confirmed via `ls
  ~/.codex/AGENTS.md` before/after the full suite run.

**Codex async primitive — verified against codex-cli 0.153.4** (`codex --help`, `codex
agents --help`, `codex queue --help`): there is no host-notified completion callback.
`codex agents` browses agent sessions on the shared local app-server daemon (a manual/CLI
browse, not a push notification); `codex queue` only queues a message into an *existing*
session. Neither is event-driven. The shipped content states this plainly and prescribes
a bounded/backoff poll against `codex agents` as the least-bad fallback, per scope item 2.

**Post-implementation verification** (scope item 3): ran `make install` then `harnez diff`
(showed only the new `~/.codex/AGENTS.md [Background Job Waiting]` addition), then `harnez
apply`. `md5sum` of `~/.claude/CLAUDE.md` and `~/.prime/agent/AGENTS.md` before/after apply
were identical (`834b00e38bcf9a6a3d9229647fd1499f` both). `~/.codex/AGENTS.md` was created
containing only the new managed section. A second `harnez apply`/`harnez diff` reported "No
changes." `harnez status` reports `agents_md:     4 global, 0 local, 1 agent`.

**Test results**: `go build ./...` clean; `go test ./...` — all packages pass, including
the 7 new `agents_profile_test.go` cases and the full pre-existing suite (`internal/claude`
0.167s, `internal/usage` 15.28s, etc.), no regressions.

**Not done here (per Out of Scope)**: issue 144's model-selection content was not migrated
into this mechanism.

**Review**: inline-reviewed diff (config.go/apply.go/status.go/config.yaml/tests/docs), reran
`go test ./... -count=1` (all green), re-verified real `~/.codex/AGENTS.md` content and
`~/.claude/CLAUDE.md` md5sum independently. Removed one out-of-scope stray file
(`update_roadmap.py`) the dev subagent left behind, unrelated to this ticket. Closed and
committed.
