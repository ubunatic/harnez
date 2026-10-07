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

- Claude → Claude: use native `SendMessage`; Harnez needs no transport of its own.
- Open: Harnez CLI → Claude (from Codex/AGY or a shell) — the socket protocol is undocumented;
  probe it separately before relying on it. Claude → Codex remains with issue 680.
