# 578 — Make harnez agent safely discoverable and usable via Bash or MCP

**Status**: Closed — generated Subagent Policy names MCP vs Bash routes, lifecycle verbs, no 'harnez advisor' (ea5a485)
**Priority**: P2
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [#575 — Add harnez_command MCP tool](575-add-harnez-command-mcp-tool-returning-cli-command-and-backgrounding-instructions.md)

---

## 1. Problem & Motivation

An agent asked to consult `terra:med` first tried an unsupported `harnez advisor` command, then inspected CLI help and config instead of discovering and using the available Harnez MCP agent tools. The user had to steer it to MCP. The agent also initially missed that `harnez agent` is the supported CLI route for model-specific agents. This required avoidable tool and command discovery before the requested review could start.

## 2. Goal

`/goal`: Any supported agent can promptly identify and safely start, inspect, wait for, and stop a Harnez agent using either the available MCP tools or `harnez agent` via Bash, without guessing command groups, parameters, or lifecycle behavior. Existing `harnez_command` coverage from #575 should be considered rather than duplicated.

## 3. Implementation & Verification Plan

Review current CLI help, MCP tool schemas/descriptions, and agent-facing guidance. Close the discoverability and invocation gaps, then verify that an agent can complete a model-specific review using one documented MCP or Bash path with correct lifecycle handling.

## Research findings (2026-09-25)

- **Premise confirmed on HEAD:** the supported paths exist, but the generated subagent policy only says to use `harnez agent`; it does not explain when to use exposed MCP tools or how to manage a session lifecycle. That leaves room for guessing an unsupported command such as `harnez advisor`.
- **Existing CLI coverage:** [cmd/harnez/agent.go](/home/uwe/projects/harnez/cmd/harnez/agent.go:52) documents model selection and short prompt forms. The lifecycle commands are implemented at `agent.go:468` (`list`), `496` (`status`), `513` (`wait`), and `573` (`stop`); resume is at `441`.
- **Existing MCP coverage:** [internal/mcp/server.go](/home/uwe/projects/harnez/internal/mcp/server.go:47) registers `harnez_spawn_agent`, `harnez_command`, `harnez_wait_agent`, `harnez_list_agents`, `harnez_agent_status`, `harnez_resume_agent`, and `harnez_stop_agent`. Their descriptions and schemas are present.
- **Existing guidance:** [docs/HarnezAgentArchitecture.md](/home/uwe/projects/harnez/docs/HarnezAgentArchitecture.md:83) explains Codex registration and tool discovery; lines 113–127 distinguish structured MCP results from host-visible Bash background jobs. Lines 135–161 cover AGY registration and discovery.
- **Likely guidance gap:** [internal/agentpolicy/policy.go](/home/uwe/projects/harnez/internal/agentpolicy/policy.go:19) generates the managed “Subagent Policy” block. Its MCP guidance is absent; the project template [docs/templates/AGENTS.md](/home/uwe/projects/harnez/docs/templates/AGENTS.md) has no separate Harnez-agent discovery instructions. The repo’s current `AGENTS.local.md` policy shows the generated wording.
- **Minimal fix:** expand `policyBody` with a short route-selection instruction: when Harnez MCP tools are exposed, use `harnez_spawn_agent` for structured direct lifecycle/results and `harnez_command` when the host should run a returned command in Bash; otherwise use `harnez agent --model <spec> --role advisor -p "<prompt>"`. Name the supported lifecycle verbs/tools and state that there is no `harnez advisor` command. Keep detailed setup and behavior in `HarnezAgentArchitecture.md`.
- **Acceptance tests:** assert generated policy for both `native` and `harnez` modes names the MCP and CLI paths, gives the correct route distinction, and does not suggest `harnez advisor`; assert MCP tool listing still exposes the lifecycle tools and required schema fields. Confirm the architecture doc’s discovery and lifecycle guidance matches those schemas and CLI forms.

## Host decision (2026-09-25)

M1: extend the generated Subagent Policy (`internal/agentpolicy/policy.go`) with a short route choice: MCP tools when exposed, else `harnez agent` via Bash; list lifecycle verbs; say there is no `harnez advisor` command; note `harnez agent wait <session>` takes the session positionally (host hit this: `--name` fails). Tests for both policy modes. No new commands.
