# 585 — harnez init docs do not explain harnez agent commands

**Status**: Closed — managed block has a short harnez agent section (MCP first, CLI forms), consistent with agentpolicy (e69f486); terra review PASS
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs

---

## 1. Problem & Motivation

After the latest `harnez init` (voxi commit `441fc53`), a Claude Code session in voxi
could see the harnez MCP tools (`harnez_spawn_agent`, `harnez_list_agents`,
`harnez_agent_status`, `harnez_wait_agent`, `harnez_resume_agent`,
`harnez_stop_agent`, `harnez_command`), but nothing in the managed docs or the
"Harnez Managed Conventions" block explains `harnez agent` CLI commands.

The managed block covers `harnez find`, `harnez issues`, `harnez index`,
`harnez read` and `harnez exec --quota-1`, but not `harnez agent`. So an agent
does not know when to use the CLI or the MCP tools, how they relate, or what
the subcommands are without probing `--help`.

## 2. Proposed Fix

Add a short `harnez agent` section to the managed conventions block (or a
bundled doc referenced from it) that covers:

- the main subcommands and one example each (spawn, list, status, wait, resume, stop);
- how they map to the `mcp__harnez__*` tools, and which one to prefer;
- how this relates to the `harnez-advisor` and `reverse-sprinter` skills.

## 3. Technical Specification / Findings

Ticket 578 already added a `Subagent Policy` block through `internal/agentpolicy`; it
establishes preferring exposed Harnez MCP lifecycle tools and falling back to the CLI
through Bash. The repo contains neither a `harnez-advisor` nor `reverse-sprinter`
skill directory, so generated guidance will omit skill references. The compact
managed subsection will link to the policy and give the six CLI forms explicitly.

## 4. Implementation & Verification Plan

Update the YAML source for the managed conventions and resync generated `AGENTS.md`
with `harnez init -d .`. Verify the generated subsection remains short and contains
the documented command forms.

## 5. Acceptance

A fresh session after `harnez init` can say how to spawn and wait for a harnez
agent from the loaded instructions alone, without running `--help`.
