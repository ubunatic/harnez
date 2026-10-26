# 963 — Teach agents to ask where rules belong before editing AGENTS.md

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: [Cati session comparison study](https://codeberg.org/ubunatic/cati/src/branch/main/docs/studies/2026-10-26-search-mcp-neus-comparison.md), cati commit `53d4914`; #285 (durable notes)

---

## 1. Problem & Motivation
In a Cati session, the owner asked the agent to compare codebase-memory MCP queries with `neus ask` and start a study. The agent turned that session instruction into an unsolicited `AGENTS.md` rule. The owner stopped it: “Don't put that into agentsmd. That's not a generic rule. This is only for this session here.” The addition was removed before commit; the study records the mistake.

Agents must not treat a useful instruction, correction, study request, or clean working tree as authorization to edit persistent agent policy. Harnez should teach destination and scope selection before writing rules.

**Goal:** Teach agents to leave `AGENTS.md` untouched unless explicitly asked or the owner selects it as the destination, preserve session-only instructions in session scope, and ask where a proposed persistent rule belongs when the destination is unclear; verify those behaviors, or stop and report when blocked on a user decision or denied permission. On loops, hard blockers, or no progress, clear the goal and record failed attempts and remaining work.

## 2. Technical Specification / Findings
When the user has not clearly specified the persistence target, ask a concise destination question with these choices:

- **a) `./.harnez/`** — repository-local Harnez guidance; distinguish shared rules from checkout-local settings using existing conventions.
- **b) `AGENTS.md`** — shared, tool-neutral repository instructions, only after explicit selection/authorization.
- **c) New issue(s) in the Harnez repository** — proposed cross-agent or harness improvements needing implementation/review, rather than immediate policy edits.
- **d) Somewhere else** — let the user name another destination, including memory. Memory is rarely used because it is generally not shared across agents; do not silently choose it.

The question concerns destination and intended scope, not filesystem cleanliness. If the destination is already explicit, act on it without asking again. If the instruction is explicitly session-only, follow it in the current session and do not persist a rule or ask to persist it. A request to create a study authorizes the study, not an `AGENTS.md` change.

Use Harnez's actual rule/skill source of truth when implementing this behavior; do not fix only one generated checkout or add a new `AGENTS.md` rule as a side effect of filing this issue.

## 3. Implementation & Verification Plan
Update the appropriate shared agent guidance and verify representative prompts:

- Session-only MCP/Neus comparison: create/update the requested study, with no `AGENTS.md`, `.harnez` policy, or memory mutation.
- Ambiguous request to retain a new rule: ask a/b/c/d before writing it anywhere persistent.
- Explicit request to put a rule in `AGENTS.md`: update that authorized destination without redundant confirmation.
- Explicit request to file a Harnez issue: file the issue, without editing agent-policy files.
- Clean checkout plus an ambiguous rule request: still ask; cleanliness does not grant edit authority.

Verify generated/installed guidance follows the source change through supported Harnez workflows and record any targeted checks. Keep the existing autonomy for ordinary implementation work; this issue narrows unsolicited persistence of agent rules.
