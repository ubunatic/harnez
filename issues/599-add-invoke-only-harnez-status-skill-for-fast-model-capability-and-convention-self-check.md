# 599 — Add invoke-only /harnez-status skill for fast model capability and convention self-check

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Skills / Workflow / Verification
**Related**: `docs/commands/`, `docs/commands/lean-sprint.md`, `docs/practices/AgenticLoop.md`, `docs/CLIDesign.md`

---

## Goal

`/goal`: Add a concise, invoke-only `/harnez-status` (or `/harnez status`) skill that lets any host model briefly self-verify and report its understanding of Harnez CLI commands, model tiers, sprint workflows, and navigation targets.

## 1. Problem & Motivation

When starting fresh sessions across different harnesses (Claude, Codex, Antigravity) or orchestrating multi-agent handoffs, human developers frequently need a fast, low-overhead way to confirm that the active model has ingested core Harnez invariants and knows how to operate without exploratory probing or hallucinatory commands.

A dedicated invoke-only `/harnez-status` skill provides a rapid self-check contract.

## 2. Technical Specification & Skill Contract

### Skill Attributes
- **Name**: `harnez-status` / `/harnez status`
- **Mode**: Invoke-only (user-invoked, brief report output).
- **Location**: `docs/commands/HarnezStatus.md` (and packaged in managed skill templates for `apply` / `init`).

### Self-Verification Checklist (to confirm briefly in output):
1. **CLI Commands**:
   - `harnez` CLI subcommands (`find`, `issues`, `read`, `usage`, `init`, `apply`, `clean`).
   - `harnez agent` lifecycle verbs (`start`, `resume`, `status`, `list`, `stop`, `wait`).
2. **Model Selection & Roles**:
   - Understands model aliases and cost hierarchy from `harnez agent models`.
   - Recognizes `luna:med` (preferred developer/worker) and `terra:med` (preferred reviewer/auditor) as low-cost worker tiers.
3. **Sprint Workflow Execution**:
   - Understands `/lean-sprint` invariants (zero-coding host, diff-first review, single-ticket pre-work batching, plan-first gate).
4. **Documentation & Roadmap Navigation**:
   - Knows where to look for `/evergreen` (`docs/*.md`), `/roadmap` (`docs/Roadmap.md`), `/issue` (`issues/*.md`), and practices (`docs/practices/`).

### Output Format Contract
- Outputs a brief, compact confirmation table/checklist (under 15 lines).
- Directly answers the 4 checklist categories without conversational filler.

## 3. Acceptance Criteria

- Skill definition added to `docs/commands/HarnezStatus.md` and registered in `config.yaml` / skill templates.
- Invoking `/harnez status` produces a concise, structured confirmation of the four capability areas.
- No side effects or file mutations on execution.
