# 2026-10-31 Sprint Retrospective: Plain Context Modes, Time-Limited Overrides, Lock Hygiene, and Detached HEAD Protection

**Date**: 2026-10-31  
**Sprint Targets**: Epic #1057 (Issue #1027, Issue #1056), Issue #1066, Issue #1065  
**Dispatched Subagents**: `searcher-1057` (`codex:luna:low`), `dev-1027` (`agy:flash38:low`), `dev-1056` (`agy:flash38:low`), `sprint-1066-dev` & `sprint-1066-dev2` (`claude:sonnet:low`), `sprint-1066-rev` (`claude:opus:low`), `dev-1065` & `dev2-1065` (`agy:flash38:low`), `sprint-1065-rev` (`agy:sonnet:low`), `refine-1068-sol` (`codex:sol:med`)  
**Host Orchestrator**: `gemini-3.8-flash-thinking` (Zero-Coding Invariant strictly observed)

---

## 1. Executive Summary & Deliveries

Today's intensive agentic pairing delivered four major capabilities into production, merged across Codeberg Pull Requests [#44](https://codeberg.org/ubunatic/harnez/pulls/44), [#45](https://codeberg.org/ubunatic/harnez/pulls/45), and [#46](https://codeberg.org/ubunatic/harnez/pulls/46):

1. **Selectable Agent Context Modes (`--context-mode`) — Issue #1027**:
   - Introduced `default`, `no-skills`, and `plain` context modes in `spec/agent.yaml`.
   - `plain` mode enforces positive whitelisting of the 6 core coding tools (`[read, edit, write, grep, find, bash]`) while cleanly stripping skills (`--no-skills`), context files (`--no-context-files`), and MCP tools (`--no-mcp`), dropping thousands of initialization tokens for small-window models and targeted tasks.

2. **Temporary, Time-Expiring Model Tier Defaults — Issue #1056**:
   - Implemented `harnez agent models set` (and alias `harnez models set`) allowing operators to temporarily redirect effort tiers (`cheap`, `lean`, `smart`) with an expiration timer (`--until=HH:MM` or `--hours=<float>`).
   - Overrides persist in XDG state (`$XDG_STATE_HOME/harnez/models/tier_overrides.json`) and are automatically resolved by subagent drivers until expiry, with optional minimal plain-mode `--hello` probes.

3. **Issue Tracker Lockfile Hygiene & Clean Fresh Clones — Issue #1066 (from Codeberg #41)**:
   - Eliminated dirty lockfile leaks in fresh clones by adding `/issues/README.md.lock` and `/issues/.reserve.lock` directly to the tracked `.gitignore` upon `harnez init`.
   - Hardened `Reserve` and `lockReadme` to safely verify inode identity via `os.SameFile` across contention loops and unlink unheld locks on exit, backed by concurrent race tests and fresh-clone integration tests.

4. **Actionable Detached HEAD Protection in Checkouts — Issue #1065 (from Codeberg #40)**:
   - Added early turn-1 detached HEAD detection in `harnez hook claude-instructions` (`SessionStart` hook), alerting agents immediately if their working tree is detached at a commit or tag.
   - Upgraded `gitflow.GuardCommit` to intercept commits on detached HEADs and provide concrete recovery commands (`git switch -c <topic> or git switch <branch>`), preventing accidental orphaned commits in shared sibling checkouts.

5. **Advanced Feature Specification & Role Profiles — Issue #1068**:
   - Formulated declarative role profiles (global in `~/.config/harnez/` and repo-local in `.harnez/`) to pin specific documentation cards, skills, and tools to agent roles on top of plain context modes.

---

## 2. Multi-Vendor Agent Effectiveness & Quota Dynamics

### Cross-Vendor Performance Spectrum
- **Gemini 3.8 Flash (`agy:flash38:low`)**:
  - Outstanding price-to-performance ratio for mechanical development and refactoring. Delivered Issue #1027, Issue #1056, and Issue #1065 cleanly and verified against full test suites without hallucinations.
- **Claude Sonnet 5.5 (`claude:sonnet:low` / `agy:sonnet:low`)**:
  - Extremely reliable on nuanced Go concurrency mechanics (resolving the `flock` and `os.SameFile` recheck logic) and sharp as an independent reviewer on gitflow edge cases.
- **Claude Opus 5.5 (`claude:opus:low`)**:
  - Exceptional review rigor. Uncovered a subtle retry-budget starvation bug in `lockReadme` where stale-reopens burned retry attempts against `readmeLockRetries`, backing the finding with synthetic concurrency data.
- **Codex Sol (`codex:sol:med`)**:
  - High-precision ticket specification and schema authoring. Successfully stepped in when Argon was quota-exhausted to refine Issue #1068 with concrete YAML models, precedence rules, and failure edge cases.

### Quota Resets & Fallback Agility
- Running mixed-vendor sprints demonstrated the resilience of the Harnez model catalog: when Claude hit its 5-hour ceiling and Argon reached its weekly subscription cap, work continued uninterrupted by falling back to `agy:flash38:low` for development, `agy:sonnet:low` for reviewing, and `codex:sol:med` for specification refinement.

---

## 3. Durable Safeguards Added

| Safeguard | Mechanism | Location |
|---|---|---|
| **Tracked Gitignore for Locks** | `harnez init` ensures tracked `.gitignore` excludes `/issues/*.lock` | `internal/claude/init_agent_file.go`, `internal/claude/gowork.go` |
| **Inode-Verified Lock Removal** | `os.SameFile` check before unlinking held lock files on graceful exit | `internal/index/index.go`, `internal/issues/issues.go` |
| **SessionStart Detached HEAD Warning** | Stderr/Stdout turn-1 alert reporting detached ref and recovery advice | `cmd/harnez/hook.go`, `internal/gitflow/gitflow.go` |
| **Commit Guard Detached Refusal** | Blocks commits on detached HEAD in protected repos with `git switch -c topic` guidance | `internal/gitflow/gitflow.go` |
| **Cross-Repo Read Convention** | Instruction forbidding checkout of tags in shared checkouts (use `git show <tag>:<path>`) | `docs/lang/Git.md`, `docs/Git.md` |

---

## 4. Architectural Learnings & Takeaways

1. **Negative Deny vs. Positive Whitelist**:
   - The `--debloat` flag historically operated via negative blacklisting (`MinimalDeny`, `AggressiveExtraDeny`), which requires continuous maintenance as providers introduce new native tools.
   - The `--context-mode=plain` approach shifts the paradigm to **strict positive whitelisting** (`tools: [read, edit, write, grep, find, bash]`). This guarantees a zero-noise execution sandbox regardless of what unmanaged tools the underlying provider adds.

2. **Flock & File Removal Semantics**:
   - Deleting a lock file under advisory `flock` on POSIX systems can create a race condition where a secondary process opens and locks a newly unlinked inode while another process creates a fresh file on disk.
   - Re-verifying `os.SameFile(fi, pi)` after acquiring the lock and bounding stale reopens (`readmeMaxStaleReopens = 10`) proves to be the canonical pattern for clean lock cleanup without leaving residual sidecars.

3. **Zero-Coding Host Orchestration**:
   - Adhering to the zero-coding invariant during `/lean-sprint` and `/sprint` allowed the host to focus entirely on requirement verification, independent review orchestration, and remote PR lifecycle management, yielding rapid parallel throughput across four distinct tickets.
