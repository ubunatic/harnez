# 1050 — Epic: Pi background task experience and reliability

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Epic
**Related**: 1011, 1021, 1044, 1046, 1047, 1048, 1049, 1051, 1052

## Goal

/goal Deliver a reliable and concise Pi background-task experience for the user and host agent, with the linked tasks verified or explicitly parked with reasons; or stop and report when blocked on user input or denied permission. On a worker loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report describing attempts, cause and resume point.

Re-check live code, ticket status and recent history before work. Existing tickets are candidates, not yet refined sprint assignments. Do not claim runtime delivery without the relevant live Pi canary.

## Scope and decisions

Task labels, output capture and presentation, completion delivery, manual command backgrounding and reload safety. Out of scope: reviving tasks across reloads, unrelated generated-index drift (1045), and changes to Pi or other external repositories. Investigate Pi APIs and existing behavior before choosing mechanisms; Ctrl-B's existing cursor binding is intentionally displaced for the requested workflow (1048). Separate host-visible content from UI rendering and protect sensitive output.

## Remaining work and delivery phases

This epic owns the remaining Pi background-task plugin work in 1021, 1044, 1046, 1047, 1048, 1049 and 1052; the MVPs in PR #31 do not close their follow-ups. Revalidate each ticket against live code and recent Pi releases before dispatch. Sprint assignments and exact write sets remain subject to discovery and refinement.

- **Capture and presentation:** 1021 and 1046, code follow-ups after the merged MVP. Review retention, truthful capture status and user/model presentation together where they overlap.
- **Completion delivery:** 1044 and 1047, code follow-ups dependent on the capture retention contract. Preserve FIFO within batches and acknowledge Pi's no-delivery-ack limitation.
- **Installation verification:** 1052, after the integrated changes to be installed are verified. Use only the targeted updater and report the credential-dependent model canary separately.
- **Pi API-dependent interaction:** 1048 and 1049 are blocked, not presumed implemented. Recheck supported APIs or seek an owner-approved alternative before scheduling.

Each code task requires its tests, install and a runtime canary before closure; design/discovery work requires a study and decomposed backlog instead. A phase exits with its children closed or explicitly parked, its declared canary or design review passing, and a clean tree.

| Ticket | What | Group | Searcher → architect → developer | Reviewer | Status |
| --- | --- | --- | --- | --- | --- |
| 1011 | Labeled task status and live verification | Labels | advisor cheap → none → developer lean | Host verification | Closed: real model tool call, interactive canary and Quota-1 passed |
| 1021 | File-backed output and selective host retrieval | Capture | advisor → architect → developer | sol | Open; MVP merged, follow-ups remain |
| 1044 | Aggregate completion notifications | Delivery | advisor → architect → developer | sol | Open; MVP merged, follow-ups remain |
| 1046 | Concise user and agent previews | Presentation | developer | sol | Open; MVP merged, follow-ups remain |
| 1047 | Safe completion delivery during compaction | Delivery | advisor → architect → developer | sol | Open; partial mitigation merged, no delivery acknowledgement |
| 1048 | Ctrl-B for manual command backgrounding | Interaction | API discovery | — | Blocked: Pi 1.4.0 has no supported running-shell transfer API |
| 1049 | Guard reload while tasks are running | Lifecycle | API discovery | — | Blocked: Pi 1.4.0 has no cancellable pre-reload hook |
| 1051 | Concise background-task launch command display | Launch presentation, independent | advisor cheap → architect smart → developer lean | reviewer smart | Closed; global refresh parked in 1052 |
| 1052 | Safely refresh global Pi task extension | Installation | advisor → none → developer | reviewer | Open; installed and ledger verified, real Claude Pi model canary pending credentials |
| — | Evergreen: study, review learnings and linked follow-ups | Final | TBD | TBD | Pending |
| — | Wrap: verify task outcomes and report next command | Final | TBD | TBD | Pending |

## Session handoff

PR #31 merged the capture, batching, presentation and compaction mitigation MVPs. Full `make test-q1` passed on the integrated product tree before the final merge of an issue and index update; isolated Pi 1.4.0 canaries exercised capture, compaction batching and interactive presentation. Sol's final review found no new concrete serious blocker. Tickets remain open for their recorded follow-ups; this is not a claim that the epic is complete.

The global Pi extension has not been refreshed with the later integrated changes. Use the targeted 1052 updater after appropriate verification, not broad `harnez apply`; its installed-copy model canary remains pending credentials. Pi supplies no delivery acknowledgement, so exactly-once delivery is not established. Resume 1048/1049 only if the relevant Pi APIs change or an owner approves an alternative; see their blocked reasons.

## Refinement and gates

Prior-art web fanout was skipped for this internal Pi integration; the installed Pi implementation and real-Pi canaries were inspected instead. Current completion messages are bounded batches, not full captured-output dumps; file capture and its retention/security limits are documented in `docs/PiAgent.md`. Recheck current Pi APIs, ticket state and model availability before resuming rather than relying on the historical session's quota or model availability.

Confirm task boundaries and ordering, especially shared output/notification behavior, after live code discovery. Record phases, dependency gates, write sets, roles and build-versus-reuse decisions before dispatch; assess whether external prior-art research adds value or is unnecessary for Pi-specific integration. Each phase exits when its children are closed or explicitly parked, its canary or design review passes, and the working tree is clean. Finish with evergreen and wrap; close only when all tasks are closed, skipped with reasons or parked in their own tickets.
