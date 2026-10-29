# 1009 — Install Pi background task runner globally and clarify MCP discovery

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 1003, 964; `docs/PiAgent.md`

---

## Problem

In a Pi session in `sitegen`, the agent incorrectly said the code-memory MCP was unavailable rather than checking its exposed tools. `list_projects` worked, but `index_status` did not auto-index the current repository; explicit `index_repository` succeeded. Improve agent guidance to discover and use the available code search MCP, check indexing, and distinguish an unavailable tool from an unindexed project. Do not assume auto-indexing without verification.

The same session could not run `/harnez-selftest`: its background task tools were absent. Harnez's generic Pi task runner is project-local under `.pi/extensions/` in the Harnez repository, while `harnez apply` installs the self-test skill globally but not that runner. Shell backgrounding did not preserve the self-test session. Raw Pi lacks this task runner.

## Goal and acceptance

/goal Make the generic Pi task runner available across projects via `harnez apply`, improve MCP discovery instructions, and guide agents having background-task trouble to run the Harnez self-test once; verify the end-to-end experience, or stop and report a blocked user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with the resume point.

- Install and manage the runner and its required dependencies/spec globally, with appropriate status, revert, and idempotency behavior; avoid conflicting project-local copies. Verify in a trusted Pi session outside the Harnez repository, including completion notification and self-test progression.
- Provide agent instructions for identifying and using exposed code-memory/search MCP tools, checking the project index and indexing when needed, with an honest fallback if tools are unavailable.
- Add an appropriate managed agent instruction: if background tasks are troublesome, run the Harnez self-test once to learn the host's task workflow; report unavailable tools rather than guessing.
- Before work, check live code, configuration and recent commits; verify the reported behavior remains current.

## Handoff (2026-10-29)

Global managed installation, duplicate coordination, MCP guidance and Pi documentation are implemented and independently reviewed. `make lint` and `make test-q1` passed; the host self-test passed with a shared explicit `--session` (instruction defect tracked in 1012). The user's normal `harnez apply` installed the runner globally and `harnez status` reported its files clean.

The user subsequently confirmed that, in another Pi session outside the Harnez repository, background tasks and the self-test worked as expected, and the agent continued after task completion. This supplies the outstanding live acceptance evidence. An earlier isolated real-loader/RPC canary exposed commands but had no authenticated model and did not establish that behavior.

A prior attempt to redirect only some apply targets inadvertently changed real home artifacts; the user subsequently ran `make install && harnez apply`, and managed status reported the affected artifacts clean. Isolate all write targets, not just the named provider targets, before any future apply canary.
