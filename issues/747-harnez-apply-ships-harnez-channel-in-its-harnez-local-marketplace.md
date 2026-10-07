# 747 — harnez apply ships harnez-channel in its harnez-local marketplace

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Bug
**Related**: 743 (plugin route), 746 (compose), internal/claude/jev_compaction.go

---

/goal `harnez apply` installs and enables the harnez-channel plugin from its own managed `harnez-local` marketplace (beside jev-compaction), so `claude --channels plugin:harnez-channel@harnez-local` starts the channel in any directory after apply, with no manual `/plugin marketplace add`; unit-tested and installed, or stop and report when blocked on a user decision.

## 1. Problem
743 added a repo marketplace `plugins/harnez-local/` and the user added it with `/plugin marketplace add`.
`harnez apply` already owns a marketplace of the same name for jev-compaction
(`internal/claude/jev_compaction.go`, embedded assets `jevcompaction/`). The next apply pointed
`known_marketplaces.json` back at `~/.claude/plugins/marketplaces/harnez-local`, whose
marketplace.json lists only jev-compaction. Result (live, 2026-10-07, psync): channels notice
shows, `/mcp` lists no `harnez-channel`, nothing registers. Enabled flag and cache stayed intact.

## 2. Fix
- Apply's embedded marketplace lists both plugins; apply writes the harnez-channel plugin files
  (plugin.json, .mcp.json running `harnez channel`) into marketplace dir and cache, records it in
  installed_plugins.json and enables it, the same way as jev-compaction. Keep the code generic
  (a list of managed plugins), not a second copy-paste of the jev block.
- Single source for the plugin files: the embedded assets. Remove `plugins/harnez-local/` from the
  repo and update `harnez channel setup` and docs (AgentMessaging.md, compose help) to drop the
  marketplace add/install steps; setup says "run harnez apply".
- Idempotent: second apply reports no changes. Existing user installs are repaired.

## 3. Verification
- Unit tests against a temp target: marketplace.json lists both plugins; channel cache files exist;
  installed and enabled entries; second run no changes; a known_marketplaces entry pointing at
  another dir is repaired.
- `make install && harnez apply`; host checks `~/.claude/plugins` state; user restarts a session
  in a non-harnez directory and the channel registers.
