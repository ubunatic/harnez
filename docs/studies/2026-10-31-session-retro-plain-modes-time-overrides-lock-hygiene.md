---
kind: retro
title: "2026-10-31 Sprint Retrospective: Plain Context Modes, Time-Limited Overrides, Lock Hygiene, and Detached HEAD Protection"
weight: 10
---

# 2026-10-31 Sprint Retrospective: Plain Context Modes, Time-Limited Overrides, Lock Hygiene, and Detached HEAD Protection

**Date**: 2026-10-31  
**Sprint Targets**: Epic #1057 (Issue #1027, Issue #1056), Issue #1066, Issue #1065  
**Dispatched Subagents**: `searcher-1057` (`codex:luna:low`), `dev-1027` (`agy:flash38:low`), `dev-1056` (`agy:flash38:low`), `sprint-1066-dev` & `sprint-1066-dev2` (`claude:sonnet:low`), `sprint-1066-rev` (`claude:opus:low`), `dev-1065` & `dev2-1065` (`agy:flash38:low`), `sprint-1065-rev` (`agy:sonnet:low`), `refine-1068-sol` (`codex:sol:med`)  
**Host Orchestrator**: `gemini-3.8-flash-thinking` (Zero-Coding Invariant strictly observed)

---

## 1. Context

This session addressed four high-leverage challenges across agent context hygiene, temporary model steering, tracker concurrency, and multi-agent git safety:
- Small-window models (such as local 32k/8k models or Pi instances) were drowning in injected initialization tokens from user/project skills, rules, and MCP schemas.
- Setting temporary model defaults for specific effort tiers required editing static YAML specs or manually passing verbose flags on every subagent start.
- Automated CI and fresh clones running `harnez issues` were accidentally staging untracked flock sidecars (`issues/README.md.lock`, `issues/.reserve.lock`) because ignores were only placed in `.git/info/exclude`.
- Agents performing cross-repo tag reads left shared checkouts on detached HEADs, causing subsequent agent sessions to commit onto detached commits and orphan work.

The goal was to execute lean sprints following the 5-phase agentic loop (Discovery → Dev → Independent Review → Process Hygiene → Retro) with a strict zero-coding host orchestrator.

---

## 2. Outcome

Four deliverables were designed, verified with tests, and merged via Codeberg Pull Requests [#44](https://codeberg.org/ubunatic/harnez/pulls/44), [#45](https://codeberg.org/ubunatic/harnez/pulls/45), and [#46](https://codeberg.org/ubunatic/harnez/pulls/46):

1. **Selectable Agent Context Modes (`--context-mode`) — Issue #1027**:
   - Added `default`, `no-skills`, and `plain` context modes in `spec/agent.yaml`.
   - `plain` mode enforces positive whitelisting of the 6 core coding tools (`[read, edit, write, grep, find, bash]`) while cleanly passing `--no-skills`, `--no-context-files`, and `--no-mcp`.

2. **Temporary, Time-Expiring Model Tier Defaults — Issue #1056**:
   - Implemented `harnez agent models set` (and alias `harnez models set`) allowing operators to temporarily redirect effort tiers (`cheap`, `lean`, `smart`) with an expiration timer (`--until=HH:MM` or `--hours=<float>`).
   - Overrides persist in XDG state (`$XDG_STATE_HOME/harnez/models/tier_overrides.json`) and are automatically resolved by subagent drivers until expiry, with optional minimal plain-mode `--hello` probes.

3. **Issue Tracker Lockfile Hygiene & Clean Fresh Clones — Issue #1066 (Codeberg #41)**:
   - `harnez init` now places `/issues/README.md.lock` and `/issues/.reserve.lock` directly into the tracked `.gitignore`.
   - `Reserve` and `lockReadme` verify inode identity via `os.SameFile` across contention loops and unlink unheld locks on graceful exit, verified with concurrent race tests and fresh clone tests.

4. **Actionable Detached HEAD Protection — Issue #1065 (Codeberg #40)**:
   - Added early turn-1 detached HEAD detection in `harnez hook claude-instructions` (`SessionStart` hook).
   - Upgraded `gitflow.GuardCommit` to intercept commits on detached HEADs and provide concrete recovery commands (`git switch -c <topic> or git switch <branch>`), preventing accidental orphaned commits in shared sibling checkouts.

5. **Advanced Feature Specification & Role Profiles — Issue #1068**:
   - Formulated declarative role profiles (global in `~/.config/harnez/` and repo-local in `.harnez/`) to pin specific documentation cards, skills, and tools to agent roles on top of plain context modes.

---

## 3. What Went Wrong

1. **Flock Inode Starvation in Concurrent Retries**:
   - *Failure*: In commit `2fbe34aa`, reopening after detecting a deleted lockfile (`!os.SameFile`) decremented the shared `attempt` counter in `lockReadme`. Under high concurrency (8+ workers), this burned all attempts and returned false "lock held by another process" errors.
   - *Detection*: Caught by independent reviewer `sprint-1066-rev` (`claude:opus:low`) during synthetic stress testing (showing 47 failures in 1600 cycles).
   - *Fix*: Separated stale-file reopens into a dedicated counter (`readmeMaxStaleReopens = 10`), preserving the full backoff retry budget for real lock contention.

2. **Hardcoded Branch Assumption in Guard Guidance**:
   - *Failure*: Initial detached HEAD guidance suggested `git switch main`. Repositories using `master` or feature topic branches received misleading commands.
   - *Detection*: Caught by independent reviewer `sprint-1065-rev` (`agy:sonnet:low`).
   - *Fix*: Updated guidance across `GuardCommit` and `CheckDetachedHead` to recommend `git switch -c <topic> or git switch <branch>`.

3. **Weekly Quota Exhaustion & Dynamic Model Pivoting**:
   - *Failure*: Claude 5-hour limit and Gemini 4 Argon weekly subscription quota were exhausted during peak development turns.
   - *Detection*: Caught by CLI pre-flight quota checks (`harnez agent start` returning error 1).
   - *Fix*: Seamlessly pivoted to `agy:flash38:low` for mechanical development, `agy:sonnet:low` for review gates, and `codex:sol:med` for specification refinement.

---

## 4. Learnings

- **Strict Positive Whitelisting vs. Negative Deny Lists**: Negative debloating (`--debloat`) requires continuous maintenance as upstream providers release new native tools. `--context-mode=plain` establishes a durable invariant by whitelisting only the necessary coding tools.
- **Flock Lifecycle & Deletion Invariant**: In POSIX systems, unlinking a file while holding an advisory `flock` requires subsequent acquirers to re-verify `os.SameFile(fi, pi)` after acquiring the lock to avoid holding orphaned descriptors.
- **Reviewer Diversity**: Opus excels at deep concurrency and edge-case modeling; Sonnet excels at API contract validation, ergonomics, and test completeness.

---

## 5. Follow-Up Tickets

The following actionable follow-ups were identified and recorded:

- **[1068](file:///home/uwe/projects/harnez/issues/1068-adopt-plain-mode-for-general-tasks-and-create-role-profiles-to-pin-docs-skills-and-tools.md)** — Adopt plain mode for general tasks and create role profiles to pin docs, skills, and tools (Filed and refined in this session).
- **[1064](file:///home/uwe/projects/harnez/issues/1064-harnez-issues-import-command-to-fetch-remote-forge-issues-into-local-tracker.md)** — Add `harnez issues import` command to fetch remote forge issues into local tracker.
- **[1067](file:///home/uwe/projects/harnez/issues/1067-add-harnez-issues-export-command-to-export-local-issues-to-remote-forge-tracker-via-agit-or-fj.md)** — Add `harnez issues export` command to export local issues to remote forge tracker via AGit or fj.
- **[1042](file:///home/uwe/projects/harnez/issues/1042-use-merge-union-in-gitattributes-for-generated-issue-indexes-to-avoid-forge-merge-conflicts.md)** — Validate live forge behavior of `merge=union` in `.gitattributes` during PR merges.
