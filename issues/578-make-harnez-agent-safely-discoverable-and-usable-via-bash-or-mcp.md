# 578 — Make harnez agent safely discoverable and usable via Bash or MCP

**Status**: Open
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
