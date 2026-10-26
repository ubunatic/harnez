# 967 — Verify harnez applies all concepts from the agent harness video (XJuLcDxtTz0)

**Status**: Open — concepts mapped by keyword search; code checks pending
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Research
**Related**: 763 (study of another harness video), 619 (Phantom Looper concepts), 914 (Harness2 study)

---

/goal Every concept the video https://www.youtube.com/watch?v=XJuLcDxtTz0 ("What Is an Agent Harness? How AI Coding
Agents Work") teaches is listed, and each is marked as applied in harnez (with the file or command that shows it),
partly applied, or missing; missing items are filed as their own tickets. Done when that table is in this ticket.
Or stop and report when no transcript or description can be read. Circuit breaker: on a loop, a hard blocker or no
progress, clear the goal and write a failure report here.

## 1. Problem & Motivation
The owner wants to know whether harnez covers the ideas in this talk on agent harnesses. Harnez should apply the
concepts it teaches, and any gap is a candidate for a ticket.

## 2. Technical Specification / Findings
The owner pasted the talk transcript (video 0:00 to 10:43, "What Is an Agent Harness?"). Its concepts, mapped to
harnez by a keyword search of `docs/` and `issues/` only (not yet read in code):

| Concept from the talk | harnez evidence | Status |
|---|---|---|
| Agent = model + harness | `docs/HarnezAgentArchitecture.md` (command surface, lifecycle) | Applied |
| Loop: call the model until it stops asking for tools | `harnez agent start/resume` drive turns; loop internals not checked | Partly (not verified) |
| Tools with name, description, schema | Harness-native tools per agent CLI; MCP not checked | Not verified |
| Context: what the model sees each step | `docs/HarnezAgentArchitecture.md` §4.1 auto-compaction at `compact_threshold_tokens` (default 200,000) | Applied |
| Context: skills load only when needed | Skills are explicit-only unless `--auto` (`harnez skill`) | Applied |
| Context: sub-agents with fresh context returning a summary | `--role advisor|developer|reviewer` leaf workers (AgenticLoop rules) | Applied |
| Memory: progress kept in files, git log as memory | `issues/` tracker and commits per milestone (`docs/AgenticLoop.md`) | Applied |
| Control: permissions before each tool run | `docs/Permissions.md`; PreToolUse hooks rewrite commands | Applied |
| Control: time and resource limits | `harnez exec` 60s default timeout and process-group kill (issue 537) | Applied |
| Sandbox | Issue 554 (quota-1 tests in a bwrap sandbox); not a general agent sandbox | Partly |
| Prompt injection defence | No evidence found in `docs/` or `issues/` | Missing (to check) |
| Doom-loop detection (same failed edit repeated) | Issue 466 covers Stop-hook loops only | Partly |
| Verify work before the agent finishes | Review gates and quota-1 runs in the lean sprint flow | Partly |
| Step limit on the agent loop, safety net | Not searched in code | Not verified |
| Ralph loop (restart with fresh context, files as memory) | Issue 706 proposes a feature-loop skill; no shipped loop found | Partly |
| Workflow vs agent: draw the flowchart first | Lean sprint is a fixed workflow; no explicit rule found | Partly (to check) |
| Harness engineering: logs, fix, re-test on each new model | Telemetry and session store; model tiers in 942/956 | Partly |
| Delete harness fixes a new model no longer needs | No rule found | Missing (to check) |

## 3. Implementation & Verification Plan
1. Done: the concept list from the transcript.
2. Next: read the code paths behind the "not verified" and "to check" rows (agent turn loop, step limit, error
   results, injection handling), then update each row with a file or command.
3. For each row still Missing after that, file one ticket and link it here.
