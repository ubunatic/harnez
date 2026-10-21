# 764 — apply: user-level jev_compaction_enabled in local.yaml

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 646 (jev compaction plugin), 747 (harnez-local marketplace), docs/JevCompaction.md, internal/claude/apply.go, internal/usage/localconfig.go

---

## Problem & Motivation

Users who install a release binary have no supported way to turn the Jev compaction plugin on or
off. `harnez apply` reads `jev_compaction_enabled` only from the `config.yaml` embedded in the
binary (default `true`), or from a full replacement file passed with `-c`. Neither
`~/.config/harnez/local.yaml` nor `~/.harnez/config.yaml` is consulted by apply. Today a user must
copy the whole config, flip the flag, and pass `-c <copy>` on every apply. Editing the repo
`config.yaml` only works for developers, who also need `make install` to re-embed it (found
2026-10-10 when disabling the plugin: the first apply after the config commit changed nothing).

`docs/JevCompaction.md` ("Opt-in: `jev_compaction_enabled: true` in `config.yaml`, then
`harnez apply`") describes only the developer path.

## Goal

/goal A user with a release binary sets `jev_compaction_enabled: false` (or `true`) in
`~/.config/harnez/local.yaml`, runs `harnez apply`, and the plugin, its `enabledPlugins` entry and
`CLAUDE_CODE_ENABLE_FUNCTION_HOOKS` are removed (or installed), while harnez-channel and the
`harnez-local` marketplace stay intact; the user value overrides the embedded default, unit-tested,
and `docs/JevCompaction.md` documents the user route; or stop and report when blocked on a user
decision (e.g. whether local.yaml is the right home versus `~/.harnez/config.yaml` or a flag).

## Notes

- Check live code first: config loading for apply may have changed since filing.
- Keep the `apply`/`init` separation (docs/CLIDesign.md): this is global, so it belongs to apply.
