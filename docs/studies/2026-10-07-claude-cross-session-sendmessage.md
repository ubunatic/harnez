# Claude Code cross-session SendMessage canary (656 M3)

Date: 2026-10-07. Claude Code 2.1.292, Linux. Issue 656 M3 (delivery discovery), Claude route.

## Probe

- Sender: interactive session `harnez1`; recipient: interactive session `harnez2`, started
  separately by the user (not a child of the sender).
- Discovery: native `ListAgents` listed peer sessions by name with state
  (`harnez2 [1e943f] · interactive · idle`).
- Send: native `SendMessage(to="harnez2", message=..., notify_when_idle=true)`.

## Result

- Send result: accepted into the recipient's inbox, explicitly "not yet read by its Claude".
- Confirmed receipt: `harnez2` replied with `SendMessage` to `harnez1`; the reply arrived as
  `<cross-session-message from="uds:/run/user/1000/cc-socks/<pid>.sock" from-name="harnez2" from-mode="prompting">`.
- Idle notice: one `[Cross-session idle notice]` arrived when `harnez2` finished its turn,
  with a harness summary of what it did.

## Semantics (from the tool contract, confirmed by this run)

- Three levels exist natively: accepted (send result), recipient processed (idle notice),
  recipient response (reply message). This maps directly onto 656 M4's
  accepted / confirmed receipt / response distinction.
- Messages drain at the recipient's next tool round; idle sessions pick them up.
- A recipient in a different permission mode may hold the message for its user's approval or
  refuse it; a `[Cross-session delivery notice]` reports that for local sessions.
- Transport is a per-session Unix socket under `/run/user/<uid>/cc-socks/`.

## Recommendation for `harnez agent send`

- Native Claude peer tools already have accepted / processed / response semantics.
- For CLI → a running Claude session, issue 740 M1 uses the documented MCP channel
  bridge below. Claude's private `cc-socks` protocol is excluded. Claude → Codex
  remains with issue 680.

## Issue 740 M1: local channel canary

Implementation date: 2026-10-07. **Live verification pending user-run sessions.**
The developer did not launch Claude. Unit tests exercise the server, transport,
registry and endpoint validation; they do not prove Claude channel delivery.

The [channel reference](https://code.claude.com/docs/en/channels-reference) documents
stdio subprocesses, the experimental `claude/channel` capability, and
`notifications/claude/channel` events. Custom servers in the research preview use
the interactive `--dangerously-load-development-channels server:<name>` flag;
`--channels` alone accepts allowlisted plugins. Development loading prompts for
consent and does not override organization policy. The
[MCP channels section](https://code.claude.com/docs/en/mcp#push-messages-with-channels)
describes session opt-in, while the
[environment reference](https://code.claude.com/docs/en/env-vars) documents
`CLAUDE_CODE_SESSION_ID` in stdio MCP subprocesses.

### Implementation and discovery

- `harnez channel` is a separate, one-way MCP server. Claude starts it as a stdio
  subprocess from the `harnez-channel` entry. The existing `harnez mcp` tool server
  and global `apply`/project `init` workflows do not install or enable this canary.
- Defaults, including `registry_dir`, bind address, bounds, model and canary text,
  come from `spec/channel.yaml` with its companion JSON Schema.
- A numeric loopback listener uses an OS-assigned port. After MCP initialization,
  the server atomically publishes a session-ID-keyed registration in the spec's
  Harnez-owned directory (currently `~/.harnez/channels`). Files are private (0600),
  the directory is private (0700), and each instance gets a random bearer token.
- `harnez channel list` lists only live PID/endpoint identity matches and omits
  tokens. `harnez channel send <session-id>` looks up that exact ID, checks the PID
  and authenticated endpoint identity, then POSTs the text. Proxies and redirects
  are disabled. Unknown IDs, stale processes/ports and non-loopback URLs fail.
- Clean stdin EOF or interrupt/termination removes the instance's own entry and
  closes the listener. Stale entries after a crash are excluded from discovery;
  restarting that session ID can replace them. Concurrent reuse of a live ID fails.
- `accepted` with `receipt: unconfirmed` means the notification was written to
  stdio. Claude does not acknowledge channel notifications and may silently drop
  them when the channel is not enabled. M1 has no receipt/reply tool or permission
  relay. Normal local permission prompts still apply.
- IDs are captured at MCP process spawn. `/clear`, implicit `--resume` and
  `--continue` can leave a subprocess holding an earlier ID; restart with an
  explicit session ID for this canary. Automatic name-to-ID routing is M2 work.

### Exact manual procedure

Run these helper commands from the Harnez checkout; they only print setup, list
registrations or send a message. They never launch Claude or write `.mcp.json`.

1. Run `scripts/canary-740-channel.sh setup`. In a scratch directory, manually
   create or merge the printed `.mcp.json` entry, preserving existing MCP servers.
2. In that scratch directory, **the user** runs the printed launch command:
   `claude --model haiku --dangerously-load-development-channels server:harnez-channel`.
   Accept development-channel and MCP consent, and confirm the channel startup
   notice. Repeat manually in another scratch directory for the second session.
3. Run `scripts/canary-740-channel.sh list` in a separate terminal. Choose the
   target `session_id` using `working_dir`; in the target, Bash can also print
   `$CLAUDE_CODE_SESSION_ID` to correlate the registration with its session.
4. **Idle:** run `scripts/canary-740-channel.sh send <session-id>`. Record the
   returned `message_id`. Observe the inbound channel event and turn, then verify
   that ID in `canary-740-receipts.txt` in the target scratch directory. This file
   records the actual tool side effect; model reply text alone is insufficient.
5. **Busy:** ask the target to run `sleep 15` through Bash. While that tool is
   running, send another default message from the separate terminal. Record when
   the inbound event appears and when the new ID reaches the receipts file.
6. **Approval:** have the target attempt a harmless scratch-directory action that
   is not preapproved in its current permission settings. While its local approval
   dialog is open, send another message. Record acceptance and whether delivery
   waits; the user answers the dialog and then checks the receipts file. This
   canary cannot approve actions remotely.
7. Exit the target normally, then run `list` again: its entry must disappear.
   Sending to its former ID must fail with `unknown channel session`. Record the
   tested Claude version, auth/policy restrictions, ID matching and observed
   idle/busy/approval behavior here before treating the live mechanism as proved.

If organization policy or feature availability prevents channel registration,
record the startup warning and stop the live probe. A successful HTTP send is
never evidence of Claude receipt by itself.

### Developer validation

- `make install` passed and updated the local binary and man pages.
- `bash -n scripts/canary-740-channel.sh` passed. The installed script's `setup`
  printed the MCP JSON and manual launch command; `list` returned an empty array.
- `make test-q1` ran once and passed (`gofmt`, `go vet`, and `go test ./...`).
  The captured output contained no `--- FAIL` lines; both `internal/channel` and
  `cmd/harnez` passed. No Go source was changed after this run.
- Live idle/busy/approval delivery and actual subprocess session-ID matching
  remain unverified until the user performs the manual procedure above.
