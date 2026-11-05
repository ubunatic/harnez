# 1129 — harnez config --global with attribution as first setting, per-repo override

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: harnez/taia#116 (taia /settings attribution per driver), `docs/CLIDesign.md` (apply vs init), `docs/Spec.md`

---

/goal Let a user set commit/PR attribution once with `harnez config`, have `harnez apply` write it into each
driver's settings, and let a repo override it; or stop and report when blocked on an owner decision.

## Problem
Claude Code adds `Co-Authored-By: Claude` and a PR footer by default. The owner wants it off: all code is
AI-written, not only by Claude. Today this needs a hand edit of `~/.claude/settings.json` (`attribution`).
A flag on `apply` would be forgotten on the next run; a stored setting is not.

## Design (proposed)
- `harnez config --global attribution off|on|"<custom text>"` stores the value in the global harnez config;
  `harnez apply` maps it to `~/.claude/settings.json` `attribution` (other drivers: what each supports).
- Per repo: `harnez config attribution ...` (no `--global`) writes `.harnez/repo.yaml`. Repo wins over global,
  because attribution can be repo policy (some projects require a trailer, some forbid it).
- Not every setting fits per repo. Each key declares its scopes in `spec/` (`global`, `repo` or both);
  `config` refuses a key at a scope it does not allow. Attribution: both. Machine-only keys (paths,
  credentials, quota): global only.
- How a repo value reaches Claude: `init` writes repo-local `.claude/settings.json` (project scope, which
  Claude Code layers over the user scope). Unverified: that `attribution` is honoured at project scope;
  probe first (canary), per `docs/CLIDesign.md` this belongs to `init`, not `apply`.

## Done when
- `config --global` and repo `config` set, show and unset `attribution`; scope rules come from `spec/`.
- `apply` and `init` write it; tests cover off, on, custom, and repo-over-global.
