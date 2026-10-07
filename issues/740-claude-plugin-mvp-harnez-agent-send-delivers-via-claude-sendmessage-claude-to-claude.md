# 740 — Claude plugin MVP: harnez agent send delivers via Claude SendMessage (Claude to Claude)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics / Feature
**Related**: 656 (`harnez agent send`, parent), 680 (Codex route), 646 (Claude function-hook plugin precedent)

---

/goal Ship a minimal Claude Code plugin so that `harnez agent send --name <session> -m <msg>`, run from a Claude session, delivers to another Claude session through native `SendMessage`, with accepted / receipt / reply reported separately and verified live between two interactive sessions; or stop and report when blocked on an unavailable plugin capability, a user decision, or a denied permission.

## 1. Problem & Motivation
The main goal of 656 is that sessions can talk to each other. Claude-to-Claude messaging
already works natively (canary 2026-10-07: `harnez1` → `harnez2` via `ListAgents` +
`SendMessage`, reply and idle notice received; see
`docs/studies/2026-10-07-claude-cross-session-sendmessage.md`). Harnez has no command that uses
it, so `harnez agent send` should route Claude → Claude through it first, as the MVP.

## 2. Technical Specification / Findings
- Native contract: send result = accepted (in inbox); `notify_when_idle` = recipient processed a
  turn; reply `<cross-session-message from-name=...>` = response. Maps onto 656 M4's three levels.
- Recipient sessions in another permission mode may hold or refuse messages; a
  `[Cross-session delivery notice]` reports it.
- Open: how a CLI command (`harnez agent send`) reaches `SendMessage`, which is a model tool, not
  a CLI. Candidates: a plugin (hook/command) that has the host Claude call `SendMessage`; or the
  plugin speaking the per-session socket in `/run/user/<uid>/cc-socks/` (undocumented — canary
  first). Decide by probe, not docs.
- Out of scope: Codex/AGY routes (680), opt-in form decision (656 §3) beyond a default-off toggle.

## 3. Implementation & Verification Plan
- Canary the plugin mechanism (load the `plugin-authoring` skill first) and record the result in
  the 656 study.
- Implement `harnez agent send` for Claude targets; report unsupported providers explicitly.
- Verify live: send between two interactive Claude sessions, show accepted, idle notice and reply.
