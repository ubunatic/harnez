# Agent Messaging

How sessions talk to each other. Evidence: `docs/studies/2026-10-07-claude-cross-session-sendmessage.md`.
Tickets: 656 (parent), 740 (Claude channel + `agent send`, closed), 742 (recipient setup), 680 (Codex).

## Rule: tickets carry the work, messages nudge

Main decisions and work requests go into tickets (any repository). A message only points the
other agent at them ("look at ticket N in repo X"). This keeps a track record between agents
without transcribing their work.

## Routes per sender → recipient

| Sender → recipient | Route | Status |
|---|---|---|
| Claude → Claude | native `ListAgents` + `SendMessage` | proven; accepted / idle notice / reply |
| CLI, Codex, AGY → Claude | `harnez agent send` → `harnez channel` MCP channel | proven (740) |
| any → Codex | Codex thread messaging | open (680) |

Not used: hooks (inject only on prompt/tool events, no idle wake-up), plain MCP (cannot push),
SDK `--resume` (stopped sessions), Claude's private `cc-socks` sockets (undocumented).

## Claude channel design (740)

- `harnez channel` is a stdio MCP server declaring `claude/channel`. Claude starts it from a
  `.mcp.json` entry; the session opts in with
  `claude --dangerously-load-development-channels server:harnez-channel` (research preview).
- Each server listens on a loopback ephemeral port with a random bearer token and registers itself
  in `~/.harnez/channels` (path in `spec/channel.yaml`), keyed by `CLAUDE_CODE_SESSION_ID`.
- `harnez agent send --name <name> | <id> -m <msg> [--wait]` resolves names from harnez records and
  Claude session metadata (names and ids only; session file content never leaves harnez), prefixes
  the sender, and reports `accepted` (transport write) then, with `--wait`, `received` when the
  recipient calls the `reply` tool, else `unconfirmed`.

## Pitfalls

- Project `.mcp.json` servers stay "Pending approval" until accepted at startup; the channel notice
  then ends with "no MCP server configured with that name" and nothing registers.
- Messages never interrupt a running tool; they show when the session is free. Mid-turn arrivals
  may be treated as untrusted until the user says the channel is theirs (742: startup prompt).
- A session resumed across a channel-server upgrade keeps its old view of the tools; start fresh.
- Haiku needs telling that `reply` exists (tools load on demand), and each `reply` call asks for
  approval until `mcp__harnez-channel__reply` is allowed (742).
- `/rename x` may be stored with a suffix (`x-1f`); `send` lists candidates on a miss.
