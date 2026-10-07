# Agent Messaging

How sessions talk to each other. Evidence: `docs/studies/2026-10-07-claude-cross-session-sendmessage.md`.
Tickets: 656 (parent), 740 (Claude channel + `agent send`, closed), 742 (recipient setup),
743/747 (plugin route, closed), 680 (Codex), 748 (plural aliases), 749 (registry pruning).

## Rule: tickets carry the work, messages nudge

Main decisions and work requests go into tickets (any repository). A message only points the
other agent at them ("look at ticket N in repo X"). This keeps a track record between agents
without transcribing their work.

## Routes per sender → recipient

| Sender → recipient | Route | Status |
|---|---|---|
| Claude → Claude | native `ListAgents` + `SendMessage` | proven; accepted / idle notice / reply |
| CLI, Codex, AGY → Claude | `harnez agent send` → `harnez channel` MCP channel | proven (740, plugin route 747) |
| any → Codex | Codex thread messaging | open (680) |

Not used: hooks (inject only on prompt/tool events, no idle wake-up), plain MCP (cannot push),
SDK `--resume` (stopped sessions), Claude's private `cc-socks` sockets (undocumented).

## Claude channel design (740)

- `harnez channel` is a stdio MCP server declaring `claude/channel`. Claude starts it from a
  `.mcp.json` entry; the session opts in with
  `claude --dangerously-load-development-channels server:harnez-channel` (research preview).
- Each server listens on a loopback ephemeral port with a random bearer token and registers itself
  in `~/.harnez/channels` (path in `spec/channel.yaml`), keyed by `CLAUDE_CODE_SESSION_ID`.
- `harnez agent send --name <name> | --id <id> <msg> [--wait]` (`-m`/`-p` also take the message; without a target it lists live session names) resolves names from harnez records and
  Claude session metadata (names and ids only; session file content never leaves harnez), prefixes
  the sender, and reports `accepted` (transport write) then, with `--wait`, `received` when the
  recipient calls the `reply` tool, else `unconfirmed`.

## Local plugin route (743)

`harnez apply` installs and enables `harnez-channel` beside `jev-compaction` in its
managed `harnez-local` marketplace. The embedded plugin runs `harnez channel` over
stdio. Install Harnez first (`make install`), ensure it is on the PATH inherited by
Claude, then run `harnez apply`. No checkout-relative marketplace registration is needed.

Then start a fresh recipient session using the command printed by `harnez channel setup`:
`claude --channels plugin:harnez-channel@harnez-local --append-system-prompt <printed instructions>`.
Keep the startup prompt current with this shell alias:

```sh
alias claude-channels='claude --channels plugin:harnez-channel@harnez-local --append-system-prompt "$(harnez compose claude-channels-prompt)"'
```

Both setup and compose read the instructions from `spec/channel.yaml`. For the full
shell-quoted launch arguments, use the function shown in `harnez compose --help`:

```sh
claude_channels() {
   eval "command claude $(harnez compose claude-channels-args) \"\$@\""
}
```

The args snippet also names the session `<working-dir-basename>-<4 random chars>`
(`claude -n`), so `harnez agent send --name` can reach it; a later `-n` overrides it.
The args snippet requires shell evaluation to preserve the prompt as one argument;
plain unquoted command substitution splits it into words.

For unattended receipts, allow `mcp__plugin_harnez-channel_harnez-channel__reply` in the
recipient project's `.claude/settings.json` permissions. The standalone server's reply rule
does not cover the plugin's namespaced tool.

The managed allowlist accepts this identity (743 section 5); packaging alone does not grant
approval. Proven live (747): a session started with the args snippet registers, and
`harnez agent send --name <name> --wait <message>` reports `received`. Restart the recipient
after binary updates.

Layout and install commands follow the [plugin docs](https://code.claude.com/docs/en/plugins)
and [marketplace docs](https://code.claude.com/docs/en/plugin-marketplaces); tool naming follows
[plugin components](https://code.claude.com/docs/en/plugins/components), and opt-in follows the
[channels reference](https://code.claude.com/docs/en/channels-reference).

## Pitfalls

- Project `.mcp.json` servers stay "Pending approval" until accepted at startup; the channel notice
  then ends with "no MCP server configured with that name" and nothing registers.
- Messages never interrupt a running tool; they show when the session is free. Mid-turn arrivals
  may be treated as untrusted without recipient trust instructions. Haiku answers in chat instead
  of calling `reply` unless the startup prompt is set (`--append-system-prompt` printed by `harnez channel setup`).
- A session resumed across a channel-server upgrade keeps its old view of the tools; start fresh.
- Models load tools on demand; each `reply` call asks for approval until `mcp__harnez-channel__reply`
  is allowed in `.claude/settings.json` (printed by `harnez channel setup`).
- Allow rules for MCP tools take the full tool name (plugin route:
  `mcp__plugin_harnez-channel_harnez-channel__reply`); Claude rejects a wildcard there as a settings error.
- A directory marketplace is read only from `<dir>/.claude-plugin/marketplace.json`; a manifest
  elsewhere makes every plugin in it "failed to load: cache-miss" (`claude plugin list`).
- `/rename x` may be stored with a suffix (`x-1f`); `send` lists candidates on a miss.
