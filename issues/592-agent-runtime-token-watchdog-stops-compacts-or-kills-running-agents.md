# 592 — agent: runtime token watchdog stops, compacts or kills running agents

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [591](591-agent-default-auto-compact-threshold-200k-tokens-for-all-agents.md)

---

## 1. Problem & Motivation
Ticket [591](591-agent-default-auto-compact-threshold-200k-tokens-for-all-agents.md) enforces
the configured token threshold before dispatching each prompt. An agent can still cross that
threshold during a long-running turn. harnez needs a runtime watchdog that can stop and compact
the session, or kill it cleanly if recovery fails.

## 2. Technical Specification / Findings
Use the global threshold and compaction verification contract defined in 591. Heartbeat data
already reports token usage. Before building watchdog behavior, run a canary for each supported
agent type (Codex, Claude, and agy) to prove that an external stop/interruption followed by a
compact request works. The control path is unproven; record canary results and any per-agent
limitations here.

On a threshold crossing, log the session name, observed token count, and time. Ask the agent to
stop its current task, then compact and verify compaction. Once verified, resume with a short
handoff prompt. If stopping or compaction does not complete within a bounded grace period, kill
the whole process group, ensure no child processes remain, and report the kill to the host.

## 3. Implementation & Verification Plan
1. Complete and record the stop-plus-compact canary for Codex, Claude, and agy before implementing
   the watchdog. Resolve or document any unsupported control path.
2. Monitor running agents and detect threshold crossings using the configured threshold.
3. Stop, compact, verify, and resume with a handoff; after the grace period, kill the process group
   and report the outcome.
4. Verify threshold detection, successful recovery, timeout/kill behavior, and process cleanup.
