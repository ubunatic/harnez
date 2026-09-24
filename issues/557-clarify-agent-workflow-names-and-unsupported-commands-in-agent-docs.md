# 557 — Clarify Agent Workflow Names and Unsupported Commands in Agent Docs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Documentation
**Related**: [#039 agentic loop practices and sprint command](039-agentic-loop-practices-and-sprint-command.md), [AgenticLoop.md](../docs/practices/AgenticLoop.md)

---

## 1. Problem & Motivation

Agents can mistake descriptive terms such as “agent run” or “agent loop” for literal supported commands. In a recent use of the docs in `../loom`, agy believed `harnez agent run` and `harnez agent loop` existed. The docs describe both Harnez's agent-session CLI and development workflows, but do not clearly distinguish command names from workflow labels or state which commands are unsupported.

## 2. Goal

Make the agent documentation unambiguous about supported Harnez agent commands versus workflow names. An agent following the docs should be able to identify the actual command to start/resume a session and understand that `/sprint` (and related workflows) are distinct from `harnez agent` subcommands. Define done as updating the canonical agent practice and any required copies or command references so no wording implies nonexistent `harnez agent run` or `harnez agent loop` commands.

## 3. Implementation & Verification Plan

- Review current CLI commands and agent docs, including the historical context in #039 and #291.
- Clarify terminology and link the CLI command list to the workflow sections; avoid describing a workflow as a CLI command.
- Check canonical and copied docs for consistency and search for misleading `harnez agent run` / `harnez agent loop` references.
