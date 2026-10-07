# 743 — Ship harnez-channel as an approved plugin channel so --channels skips the development warning

**Status**: Closed — Fixed: local plugin harnez-channel@harnez-local (48307bf9) plus user-set managed allowlist; live in repo session harnez3: clean start without development warning, registered, send --wait accepted -> received
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

## 5. Allowlist probe (user, 2026-10-07): passed

`/etc/claude-code/managed-settings.json` =
`{"channelsEnabled":true,"allowedChannelPlugins":[{"marketplace":"harnez-local","plugin":"harnez-channel"}]}`,
then `claude --channels plugin:harnez-channel@harnez-local` with no plugin installed. Startup notice:
`plugin:harnez-channel@harnez-local · plugin not installed` (not "not on approved list"). So the
local managed allowlist is honored on a personal account. Next: step 1, package and install the
plugin from a local `harnez-local` marketplace, then rerun the same launch.

## 6. Developer handoff (2026-10-07)

- Added `plugins/harnez-local/.claude-plugin/marketplace.json`, naming `harnez-local`,
  with the relative source `./plugins/harnez-channel`. The plugin manifest names
  `harnez-channel`; its root `.mcp.json` runs `harnez channel` over stdio via PATH.
- `harnez channel setup` now prints the local marketplace add/install commands,
  `claude --channels plugin:harnez-channel@harnez-local --append-system-prompt ...`,
  and `mcp__plugin_harnez-channel_harnez-channel__reply` for unattended receipts.
  Existing standalone development setup remains available. Plugin identity/path live
  in `spec/channel.yaml` with schema coverage. Updated `docs/AgentMessaging.md`.
- Tests cover printed setup, JSON parsing, marketplace/manifest identity agreement,
  wrong marketplace/entry/manifest names, and the MCP command/args.
- `harnez exec --quota-1 -- go test ./internal/channel/ ./cmd/harnez/` passed both packages.
- `make install` passed; installed `harnez channel setup` output inspected.
- User action: from a Claude session in the Harnez checkout, run
  `/plugin marketplace add ./plugins/harnez-local`, then
  `/plugin install harnez-channel@harnez-local`. Start a fresh recipient with the
  launch command printed by `harnez channel setup`; merge the plugin reply allow rule
  into recipient permissions, preserving existing rules.
- Live acceptance: no development warning, `harnez channel list` contains the recipient,
  and `harnez agent send --name <name> -m <message> --wait` returns `received`.
  No Claude launch or plugin installation was performed by the worker; managed settings
  were untouched. Ticket stays open for the host to close after the user's live test.
