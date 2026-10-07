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

## 4. Research Outcome & Milestones (host, 2026-10-07)

Research (web-740, codex:luna:med): the documented push route into a running Claude session is a
**channel** (https://code.claude.com/docs/en/channels, https://code.claude.com/docs/en/mcp#push-messages-with-channels).
The target session opts in at launch: `claude --channels server:<name>` (manually configured MCP
server) or `plugin:<name>@<marketplace>` (allowlisted). Hooks only inject on prompt/tool events
(no idle wake-up); plain MCP cannot push; SDK `--resume` targets stopped sessions; `cc-socks`
sockets are undocumented — do not use.

- **M1 Channel canary.** Minimal `harnez-channel` MCP channel server (stdio, launched by Claude as a
  `server:` entry) that listens on a per-session local endpoint and pushes received text into its
  session. Script `scripts/canary-740-channel.sh` plus setup notes. Prove: idle delivery, busy
  delivery, approval prompts, how the endpoint is found per target session. Record in
  `docs/studies/2026-10-07-claude-cross-session-sendmessage.md`.
- **M2 `harnez agent send --name <session> -m <msg>`** for Claude targets via the M1 channel;
  accepted vs receipt reported separately; unsupported providers fail explicitly. Unit tests plus
  live check by the host.
