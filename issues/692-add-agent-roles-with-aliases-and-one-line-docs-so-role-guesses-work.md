# 692 — Add agent roles with aliases and one-line docs so --role guesses work

**Status**: Closed — M1 18a910fe: spec schema, role descriptions, aliases, ResolveRole, help and error catalog table
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 686 (decider mapping for unknown roles), 510 (researcher role), 486 (roles in `spec/agent.yaml`)

---

/goal Agents pick a working `--role` on the first try: common role names
either exist as roles or map to one, and each role has a one-line
description that `harnez agent start --help` and the docs show; stop and
report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
Only four roles exist (orchestrator, developer, reviewer, advisor). Agents
guess other natural names and fail. In lmcoder on 2026-10-03, Claude started
a read-only lookup agent with `--role explorer`, got "unknown agent role",
and had to retry with `advisor`. Nothing in `--help` says what each role may
do, so the right choice is hit and miss.

## 2. Technical Specification / Findings
- Roles and their rules live in `spec/agent.yaml` `roles:`; the check is in
  `internal/subagent/agentspec.go` (error lists the known names only).
- `--help` lists the names but not what they allow.
- 686 maps unknown names through the decider (Jev) when it is available.
  This ticket is the cheap, deterministic layer under it: static aliases
  plus descriptions, which work without the decider.
- 510 already asks for a web-only `researcher` role.

## 3. Implementation & Verification Plan
- Add a `description` (one line) and optional `aliases` list per role in
  `spec/agent.yaml`. Candidate aliases: `explorer`, `scout`, `auditor`,
  `planner` → advisor; `coder`, `worker`, `implementer` → developer;
  `critic`, `checker` → reviewer; `lead`, `coordinator` → orchestrator.
- Decide per name whether it deserves its own role instead of an alias
  (`researcher` per 510; maybe `filer` for issue filing). Keep the set small.
- Show name, aliases and description in `harnez agent start --help`, in the
  unknown-role error, and in a short role table in the agent docs.
- Resolve an alias to its role and record the canonical role in the session.

Verify: `harnez agent start --role explorer ...` starts an advisor session;
an unknown name fails with the role table; `--help` shows the descriptions;
`make test-q1` passes.
