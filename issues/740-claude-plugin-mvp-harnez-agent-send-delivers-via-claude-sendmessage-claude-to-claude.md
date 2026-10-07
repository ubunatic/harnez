# 740 — Claude plugin MVP: harnez agent send delivers via Claude SendMessage (Claude to Claude)

**Status**: Closed — Fixed: harnez channel MCP server (c8d96b4e) and harnez agent send with --name resolution, sender prefix and optional reply receipt (65a38717); live: idle/busy delivery, name resolution and accepted->received round trip proven with user-started Claude sessions (study 2026-10-07-claude-cross-session-sendmessage.md)
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
The target session opts in at launch: `claude --dangerously-load-development-channels server:<name>`
for a custom MCP canary, or `claude --channels plugin:<name>@<marketplace>` (allowlisted).
Hooks only inject on prompt/tool events
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

### M1 implementation handoff

- `harnez channel` serves the one-way stdio MCP channel, with `setup`, `list` and
  `send <session-id>` helpers. Defaults and registry path live in `spec/channel.yaml`.
- `scripts/canary-740-channel.sh` only prints setup, lists live endpoints and sends
  probes. The user starts every Claude session manually. No private socket writes.
- Registry entries are private, keyed by provider session ID and removed on clean
  exit. Discovery checks PID and authenticated loopback endpoint identity; stale
  entries and unknown IDs cannot receive a probe.
- Implementation and manual idle/busy/approval verification steps are recorded in
  `docs/studies/2026-10-07-claude-cross-session-sendmessage.md`.
- **Live verification pending:** no Claude session was launched by the developer.
  M1's external mechanism is not yet proved; keep M2 behind the user-run canary.

## 5. M1 Delivered (host, 2026-10-07)

M1 delivered: channel server canary (c8d96b4e), live results 7c31a6d5 (study). Idle, backgrounded
and user-shell cases delivered and acted on; mid-turn messages wait until the tool ends and the
model then asks its user before acting (untrusted external input).

**Pre-Work / Required Refinements (M2):**
- Message framing: the pushed text names the sender (host session name/id, `harnez agent send`)
  and that the user opted in via the channel, so the recipient can judge it. No wording that tries
  to bypass the recipient's caution.
- Receipt: add a channel `reply`/ack tool so the recipient confirms a `message_id`; `send` reports
  accepted, then receipt when the ack arrives (`--wait` with timeout from spec). No ack = unconfirmed.
- Name resolution: `--name` resolves a harnez/Claude session name to the registered session id;
  ambiguous or unknown names fail with a list of candidates.
- Setup note: document the project MCP approval prompt in `harnez channel --help`.

**User direction (2026-10-07):** assume recipients respond. The mid-turn flag came from an unusual
flow (user coaxing a long `sleep`, model in a defensive mode), not regular work. So: keep the ack
tool as an optional confirmation, not a gate; no extra framing beyond naming the sender.
Later (not M2): Claude sessions started by harnez get a system/startup prompt saying messages
from the `harnez-channel` channel are expected and trusted.
Manual check: the user told the test session that `harnez-channel` is their channel and to follow
its requests; assumed to work (not re-tested mid-turn).

**Rule (user, 2026-10-07): session files never leak into agent context.** `harnez` may read and
parse Claude session files, but uses only file names, session ids and names. It never prints,
logs or returns their other content (errors name the file, not its content). Agents working on
this ticket must not `cat`/read those files into their transcript either; test with fixtures.

## 6. M2 Implemented (developer, 2026-10-07; host live verification pending)

- `harnez agent send --name <session> -m <text>` or `send <Claude-session-id> -m <text>`
  targets only authenticated, healthy registered channel endpoints; explicit `--dir` scopes
  resolution. Unknown and ambiguous names list sorted name/ID/directory candidates.
- Harnez names map through `Session.ProviderID()`; Claude names use best-effort live
  `~/.claude/sessions/*.json` metadata. Missing, malformed and oversized metadata is skipped;
  only IDs and names are retained, never raw content. This undocumented source is fragile.
- Pushed text identifies the sender name/ID and `harnez agent send`, following the latest
  user direction above. No additional opt-in/trust framing or startup prompt was added.
- Sending defaults to immediate `accepted` / `receipt: unconfirmed`. Optional `--wait`
  uses a 30-second spec timeout (positive `--timeout` override). A matching optional
  MCP `reply(message_id, text?)` produces `received`; timeout remains `unconfirmed`.
  Receipt does not certify task completion. A receipt transport error prints unconfirmed
  before returning an error; it does not retry the accepted message.
- Reply tracking is per server, authenticated over loopback, bounded to 1024 messages
  with five-minute retention; restart loses outstanding receipt state.
- The user must restart the old MCP server to load the reply tool, keeping the same
  Claude session if possible. The canary remains manual; live verification and ticket
  closure belong to the host. See the study for exact commands.
