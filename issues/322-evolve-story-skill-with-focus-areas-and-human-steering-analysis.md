# 322 — Evolve /story skill with optional focus areas, tooling fit, and human-steering divergence analysis

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `commands/story.md`, `lmcoder:docs/studies/2026-09-13-local-llm-lifecycle-session-architecture-and-agentic-velocity.md`

---

## Summary

The current `/story` skill template (`commands/story.md`) defines a rigid 8-section outline that often yields boilerplate prose and repetitive tables. In practice, the most valuable insights from real development sprints (as demonstrated in `lmcoder`'s 2026-09-13 session retrospective) come from:

1. **Human-in-the-Loop vs. Autonomous Divergence Analysis**: Comparing what was actually delivered with where an unattended agent would have diverged (e.g., premature architectural complexity, failing to recognize external service models like systemd user services, or thrashing workstation resources with unconstrained parallel container sweeps).
2. **Language & Tooling Impact**: Directly analyzing how primary language properties (e.g. Go single static binaries, sub-second test loops) and harness tooling (`harnez` index/issue tracking) accelerated or hindered progress.
3. **Optional Focus Areas**: Allowing the user to invoke `/story <3-4 word focus>` (e.g. `/story human steering and toolchain` or `/story voxi latency benchmarks`) so the agent knows what themes to emphasize without needing an exhaustive prompt.

## Proposed Changes

Update `commands/story.md` in `harnez` to replace the rigid section checklist with an **adaptive, high-signal structure**:

1. **Focus Area Argument Support**:
   - Explicitly define that the skill accepts optional 3–5 word focus arguments to guide thematic emphasis.
2. **Codify Core Reflection Sections**:
   - **Executive Summary & Delivered Milestones**: High-level achievements and closed issues.
   - **What Worked Well**: Technical wins and clean abstractions.
   - **Honest Post-Mortem (Failures, Friction & Near-Misses)**: Explicit bugs, bad assumptions, hardware contention, and resolutions.
   - **Tooling, Language & Harness Acceleration**: Concrete evaluation of language and CLI tooling fit.
   - **Human-in-the-Loop vs. Autonomous Divergence**: Contrasting human steering with unsupervised drift hazards (*"build new features and leave"*).
   - **Key Learnings & Evergreen Upstream**: Rules to promote to evergreen docs or `AGENTS.md`.
   - **File & Issue Traceability**: Clean summary table of modified files and closed issues.
3. **Encourage High-Signal Markdown**:
   - Guide the agent to use Markdown tables, ASCII contrast boxes, and code links instead of dense prose blocks.

## Verification

- [ ] `commands/story.md` updated with the adaptive structure and focus area guidelines.
- [ ] `harnez apply` deploys updated `SKILL.md` to `~/.gemini/skills/story/`, `~/.claude/skills/story/`, `~/.codex/skills/story/`, and `~/.prime/agent/skills/story/`.
- [ ] Calling `/story` or `/story <focus>` produces concise, high-signal retrospectives with the new divergence and tooling analysis.
