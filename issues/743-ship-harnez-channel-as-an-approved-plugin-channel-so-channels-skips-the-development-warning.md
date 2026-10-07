# 743 — Ship harnez-channel as an approved plugin channel so --channels skips the development warning

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 742 (recipient setup), 740 (channel), docs/AgentMessaging.md

---

/goal Start a Claude session with `claude --channels plugin:harnez-channel@<marketplace>` and no "Loading development channels" confirmation, using a documented approval route, verified live; or stop and report when the route needs an organization allowlist or a user decision.

## 1. Problem & Motivation
`--dangerously-load-development-channels server:harnez-channel` asks for confirmation on every
launch (user, 2026-10-07). `--channels` takes only `plugin:<name>@<marketplace>` entries, with an
allowlist enforced.

## 2. Technical Specification / Findings
Docs read 2026-10-07: https://code.claude.com/docs/en/channels (Enterprise controls, Research preview),
https://code.claude.com/docs/en/channels-reference (Test during the research preview, Package as a plugin).

- During the preview `--channels` accepts only plugins on the effective allowlist: by default the
  channel plugins in `claude-plugins-official`; on Team/Enterprise, an admin's `allowedChannelPlugins`
  (managed settings) replaces it. Off-list plugins start but do not register.
- channels-reference, "Package as a plugin": "A channel published to your own marketplace still needs
  `--dangerously-load-development-channels`". The community marketplace is not on the allowlist.
  Documented routes onto the list: an official-marketplace listing via an Anthropic partner contact,
  or an org admin's `allowedChannelPlugins`.
- "Pro and Max users without an organization skip these checks entirely": the solo user gets the
  default Anthropic list and has no documented way to extend it.
- So no documented route exists for a solo user. Plugin layout and local marketplaces do not matter
  until the allowlist question is solved.

Uncertain, not documented: whether `allowedChannelPlugins` in a locally written managed settings
file (for example `/etc/claude-code/managed-settings.json`) is honored for a solo Pro/Max account.
The docs tie it to Team/Enterprise admins, and "skip these checks" suggests it is ignored.

## 3. Implementation & Verification Plan
Blocked: the user decides between (a) keep the development flag (status quo, one confirmation per
launch), (b) a canary of the undocumented local managed-settings route (docs/Canary.md, with root write
to `/etc`), (c) ask Anthropic for an official listing. Nothing is built before that decision.

## 4. Decision (user, 2026-10-07): try (b), local managed-settings allowlist

Canary (docs/Canary.md), undocumented route, expected to fail; record the result either way.
1. Package `harnez channel` as a plugin in a local directory marketplace (plugin.json, the
   plugin's `.mcp.json` with the `harnez-channel` server, marketplace.json); install it with
   `claude plugin marketplace add` / `claude plugin install` as the docs describe.
2. The **user** writes the managed settings file (root needed; agents print the exact content and
   `sudo` command, never run it): `allowedChannelPlugins` naming `harnez-channel@<marketplace>`
   in `/etc/claude-code/managed-settings.json` (check the documented Linux path and key schema first).
3. The **user** launches `claude --channels plugin:harnez-channel@<marketplace>`. Pass = no
   development-channel warning, channel registers (`harnez channel list`), `send --wait` → `received`.
4. On fail: record the observed warning/error, tell the user to remove the managed-settings entry,
   and fall back to (a) (development flag, one confirmation per launch); close as won't-fix.
