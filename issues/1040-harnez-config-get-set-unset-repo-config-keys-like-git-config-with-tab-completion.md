# 1040 — harnez config: get/set/unset repo config keys like git config, with tab completion

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: [[1039]] (git.mode / git.protected_main keys), [[109]] (user-local config), spec/schemas/repo.schema.json

---

## Goal

/goal Add `harnez config` so that a user can read and change `<repo>/.harnez/repo.yaml` keys from the
command line, the way `git config` works, without editing YAML by hand. Every write is validated against
the repo schema, and shell completion offers the keys and their allowed values. Verify it in a scratch repo
for each subcommand. Stop and report when blocked on a user decision or a denied permission. On a loop, a
hard blocker, or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

Per-repo settings such as `git.mode` and `git.protected_main` (1039), `issues.tracker` and the ticket cap
live in `.harnez/repo.yaml`. The only ways to change them today are a hand edit or a re-run of
`harnez init` with flags, and neither tells you which keys and values exist. A `git config`-style command
with completion makes these settings easy to find and safe to change.

## 2. Proposed Shape

```
harnez config [-d <repo>] <key>                 # get
harnez config [-d <repo>] <key> <value>         # set
harnez config [-d <repo>] --unset <key>
harnez config [-d <repo>] --list                # effective values, defaults marked
```

- Keys are dotted paths into `repo.yaml` (`git.mode`, `issues.tracker`, ...). The key list, the types and
  the enum values come from `spec/schemas/repo.schema.json`. They must not be duplicated in Go (docs/Spec.md).
- **Validate before writing.** A set or unset runs the same checks as `LoadRepoConfig`, including the
  `gitflow` rules such as "protected_main rejected on solo". When a check fails, nothing is written, and
  the error names the key and lists the allowed values.
- **Keep the file intact.** Write through `yaml.v3` nodes so that comments, key order and unrelated keys
  survive.
- **Completion.** Cobra completion is used for fish, bash and zsh. It completes key names first. For a
  key, it then completes the enum values or `true`/`false`.
- `--list` shows the effective value of each key, with spec defaults marked (for example `git.mode=solo
  (default)`).

## 3. Open Questions (decide in the sprint)

- **Global scope:** should a `--global` flag target `~/.config/harnez/local.yaml` (from 109), the way
  `git config --global` does? Or is this repo-only for now?
- **Init overlap:** after the change, should `init --git-mode` / `--protected-main` call the same setter, or
  stay as they are?
- **Post-set hook:** after a set that changes rendered output (such as Repo Setup in AGENTS.md), should the
  command print a hint to run `harnez init -d .`, or run init itself?

## 4. Verification

- Unit tests cover get, set, unset and list. They also cover:
  - an unknown key, a bad enum value and a wrong type;
  - `git.protected_main true` on a solo repo, which must be rejected with the file unchanged;
  - comments that survive a round trip.
- A completion test runs `harnez __complete config git.` and `harnez __complete config git.mode ""`.
- A scratch repo run with the installed binary covers every subcommand. In that run, `harnez init -d .`
  leaves the file unchanged.
