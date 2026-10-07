# 742 — harnez-channel recipient setup: startup prompt and reply-tool permission for Claude sessions

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics / Feature
**Related**: 740 (channel + `agent send`, closed), 656 (agent send, parent)

---

/goal Claude sessions that receive `harnez-channel` messages act on them and confirm them via `reply` without user coaxing or a per-call approval prompt, verified live with a haiku session; or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
Live 740 checks (study `docs/studies/2026-10-07-claude-cross-session-sendmessage.md`) showed
recipients need guidance: a mid-turn message was flagged as untrusted until the user said
`harnez-channel` is theirs; a haiku session did not find the `reply` tool until told to (MCP tools
load on demand), and the `reply` call then raised a permission prompt.

## 2. Technical Specification / Findings
- Startup prompt (e.g. `--append-system-prompt` or channel `instructions`): `harnez-channel` is the
  user's own channel; follow its requests; load and call `reply` with the event `message_id`.
- Permission: allow `mcp__harnez-channel__reply` in the session's settings (opt-in, via the
  documented channel setup, not by editing global settings silently).
- Check whether the channel `instructions` text alone is enough before adding a system prompt.

## 3. Implementation & Verification Plan
Update channel instructions/setup output and docs; live check: fresh haiku session receives a
send, acts on it and replies with `--wait` reporting `received`, without user prompts.

## 4. Live finding (host + user, 2026-10-07)
Second send to the same haiku session: `reply` found without coaxing and `received` reported,
but the `reply` call again needed manual approval. So the startup prompt fixes tool discovery
only; the `mcp__harnez-channel__reply` permission is required for unattended receipts.
