# 680 — Probe Codex cross-task messaging for agent send

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 656 (`harnez agent send`)

---

/goal Probe and document Codex cross-task discovery and message delivery for #656, or stop and report when blocked by unavailable session access or a denied permission.

## 1. Problem & Motivation
Issue 656 needs a proven Codex delivery method for `harnez agent send`. In this Codex environment, native task tools can list tasks and send a follow-up by thread ID, including to an interactive task that is not a child in the current subagent tree. The send call returned the target thread ID, but that alone does not confirm the recipient processed or answered the message.

## 2. Technical Specification / Findings
The available native operations include listing tasks, reading a task by ID, sending a follow-up to a task by ID, and waiting for task updates. Task titles are labels; thread IDs address the target. Determine which supported Codex interface exposes these operations to Harnez, how named agents map to Codex thread IDs, and what the interface guarantees for active versus idle targets. Do not treat an accepted send as confirmed delivery.

**2026-10-08, user canary (Codex 0.161.0):** task A1 called `codex_tui.list_threads` and
`codex_tui.send_message_to_thread` to send "hi" to task A2; A2 showed "Sent by Codex from task <id>"
and replied the same way; A1 received the reply. Codex-to-Codex delivery and reply work natively.
These are tools of the Codex model, not of an outside process. The candidate outside entry point is
`codex queue --thread <session UUID or exact name> --message <text>` ("Queue a message for an
existing session"); `codex agents` and `codex app-server daemon|proxy` suggest a shared local
app-server daemon behind it. Unproven: whether `codex queue` reaches a running TUI live or only on
its next resume, and how to list threads non-interactively.

**2026-10-08, harnez-side canary:** from a Claude session, `codex queue --thread loom2 …` failed:
"Cannot verify a unique session label across server pages; matching session UUID: 01a11828-…".
`codex queue --thread 01a11828-… --message …` returned "Queued message … for thread …", and the idle
TUI session loom2 showed the message live and acted on it within seconds. Delivery to a running
session works. Findings for the send route:
- Address by UUID; name lookup is not reliable, so harnez must resolve name → UUID itself.
- The recipient sees no sender. loom2 answered its last Codex correspondent (loom1) instead of the
  Claude sender, and loom1 bounced "received" back once. Each message must name the sender and the
  reply command (e.g. `harnez agent send --name <sender> …`).
Still open: a non-interactive thread listing for name → UUID (candidate: the app-server protocol via
`codex app-server proxy`; check `generate-json-schema` for a thread list method).

## 3. Implementation & Verification Plan
Canary-test a harmless message to a separate Codex task, confirm its receipt or response, and record targeting, active/idle behavior, failure cases, and confirmation semantics in `docs/studies/` for issue 656. Recommend how `harnez agent send` should use the verified interface.
