# 510 — Add a web-only researcher role for harnez agent

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: `spec/agent.yaml` (roles), `docs/HarnezAgentArchitecture.md` §2.10, `docs/Models.md` (2026-09-23 snapshot)

## Problem

The 2026-09-23 model research sweep sent 11 `--role advisor` agents to do web research.
The advisor rules ("audit the ticket and code paths you are given and return a short
implementation plan") led one of them (the agy:opus researcher) to run repo commands and
draft a plan for ticket 500, which nobody asked for, spending tokens on the wrong task.

## /goal

`harnez agent start --role researcher` exists as a leaf, read-only role whose rules say:
answer from web sources, cite them, say when evidence is thin, and do not inspect the
repository unless the prompt asks for it. The role is defined in `spec/agent.yaml` only
(no Go constants), and orchestrators may spawn it.

## Done when

- The role is in `spec/agent.yaml` and in the orchestrator's `spawns` list, and
  `--role researcher` completion lists it.
- A test covers that the researcher is a leaf (it cannot start agents).
