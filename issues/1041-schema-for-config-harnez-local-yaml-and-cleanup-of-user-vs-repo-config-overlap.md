# 1041 — Schema for ~/.config/harnez/local.yaml and cleanup of user vs repo config overlap

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: [[109]] (local.yaml introduced), [[925]] (user overrides of spec values), [[764]] (user-level jev compaction in local.yaml), [[941]] (`~/.config/harnez/models.yaml`), [[1040]] (`harnez config` for repo.yaml), [[1039]] (`git:` key), docs/Spec.md

---

## Goal

/goal Give `~/.config/harnez/local.yaml` a JSON schema in `spec/schemas/`, validated on load with errors that name
the file and key, the same way `.harnez/repo.yaml` is validated. Then audit how the user-level files in
`~/.config/harnez/` relate to the per-repo `.harnez/` files and to the `spec/` defaults. Write down the precedence,
and clean up overlaps, dead keys and duplicated defaults. Stop and report when blocked on a user decision, such as
dropping or renaming a key users may already have set, or on a denied permission. On a loop, a hard blocker, or no
progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

harnez now reads config from three places with no documented relationship between them:

- **User level:** `~/.config/harnez/local.yaml` (109), loaded in `internal/usage/localconfig.go` into typed structs
  with no schema. Its known keys are `usage.default_host` and `load.watch_host`. `models.yaml` (941) lives in the
  same directory.
- **Repo level:** `.harnez/repo.yaml`, which is schema-validated (`spec/schemas/repo.schema.json`). It gained
  `git.*` in 1039 and is getting a `harnez config` command in 1040.
- **Spec defaults:** `spec/*.yaml`. 925 proposes user overrides of these from `~/.config/harnez`.

A typo in `local.yaml` is silently ignored today. No doc says which level wins, and some defaults still live in Go
(for example the `make.scaffold` default, which 1040 moves into the spec).

## 2. Scope

1. **Schema.**
   - Add `spec/schemas/local.schema.json` with `additionalProperties: false`.
   - Validate it in the loader. An unknown key or wrong type fails with the file path and the dotted key.
   - A missing file stays valid.
2. **Audit**, recorded as a table in this ticket. List every key read from `~/.config/harnez/*` and `.harnez/*`,
   with:
   - its owner (loader file:line);
   - its default and where that default lives (spec or Go);
   - whether the same setting exists at another level.
3. **Cleanup.** Act on what the audit finds:
   - move defaults that live only in Go into `spec/`;
   - remove dead keys;
   - merge settings that are duplicated across levels.
   A key that existing users may already have set needs a deprecation path or an owner decision before it is
   removed.
4. **Docs.** A short precedence section in `docs/Spec.md` or a new evergreen doc:
   - the levels and what each one is for;
   - which keys may be set at which level;
   - how `models.yaml` and the 925 overrides fit in.

## 3. Open Questions

- Should `local.yaml` and the 925 spec overrides be one file or stay separate?
- Should `harnez config` (1040) later get `--global` for `local.yaml`, once it has a schema? This only needs
  noting here, not implementing.
- Does any repo-level key belong at the user level, or the reverse? For example `git.mode` defaults or the quota
  rules.

## 4. Verification

- **Unit tests:** an unknown key, a wrong type and a missing file. A valid file loads as it does today.
- **Real config:** the current `~/.config/harnez` contents still load on this machine. Today that is
  `models.yaml` only, with no `local.yaml`.
- `make lint` and `make test-q1` show no new failures.
