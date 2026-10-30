# 1050 — Epic: Pi background task experience and reliability

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Epic
**Related**: 1011, 1021, 1044, 1046, 1047, 1048, 1049

## Goal

/goal Deliver a reliable and concise Pi background-task experience for the user and host agent, with the linked tasks verified or explicitly parked with reasons; or stop and report when blocked on user input or denied permission. On a worker loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report describing attempts, cause and resume point.

Re-check live code, ticket status and recent history before work. Existing tickets are candidates, not yet refined sprint assignments. Do not claim runtime delivery without the relevant live Pi canary.

## Scope and decisions

Task labels, output capture and presentation, completion delivery, manual command backgrounding and reload safety. Out of scope: reviving tasks across reloads, unrelated generated-index drift (1045), and changes to Pi or other external repositories. Investigate Pi APIs and existing behavior before choosing mechanisms; Ctrl-B's existing cursor binding is intentionally displaced for the requested workflow (1048). Separate host-visible content from UI rendering and protect sensitive output.

## Candidate tasks

Sprint groups, dependencies and role/model assignments are placeholders until code discovery and refinement. Each code task requires its tests, install and a runtime canary before closure; design/discovery work requires a study and decomposed backlog instead.

| Ticket | What | Group | Searcher → architect → developer | Reviewer | Status |
| --- | --- | --- | --- | --- | --- |
| 1011 | Labeled task status; verify remaining model-issued tool path | TBD | TBD | TBD | Open; implementation committed, verification pending |
| 1021 | File-backed output and selective host retrieval | TBD | TBD | TBD | Open |
| 1044 | Aggregate completion notifications | TBD | TBD | TBD | Open |
| 1046 | Concise user and agent previews, preferably output tail | TBD | TBD | TBD | Open |
| 1047 | Safe completion delivery during compaction | TBD | TBD | TBD | Open |
| 1048 | Ctrl-B for manual command backgrounding | TBD | TBD | TBD | Open |
| 1049 | Guard reload while tasks are running | TBD | TBD | TBD | Open |
| — | Evergreen: study, review learnings and linked follow-ups | Final | TBD | TBD | Pending |
| — | Wrap: verify task outcomes and report next command | Final | TBD | TBD | Pending |

## Refinement and gates

Confirm task boundaries and ordering, especially shared output/notification behavior, after live code discovery. Record phases, dependency gates, write sets, roles and build-versus-reuse decisions before dispatch; assess whether external prior-art research adds value or is unnecessary for Pi-specific integration. Each phase exits when its children are closed or explicitly parked, its canary or design review passes, and the working tree is clean. Finish with evergreen and wrap; close only when all tasks are closed, skipped with reasons or parked in their own tickets.
