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

## Decision

- Host: the open questions in §3 are decided as follows:
  - **Repo-only scope.** No `--global`. `~/.config/harnez/local.yaml` has no schema, and only 2 keys
    (`usage.default_host`, `load.watch_host`). That is cut from this ticket.
  - **Init unchanged.** `init --git-mode/--protected-main` keep `applyGitFlags`. Both paths share the
    validation (`LoadRepoConfig` rules); nothing else is shared.
  - **Hint, no auto-run.** A set or unset of a key that drives rendered output prints
    `run: harnez init -d <repo>`; the command does not run init itself. Which keys count must be read from
    spec data, not a Go list (docs/Spec.md).
- Host: there is no Architect step; the scope is one command over an existing schema. The Developer is
  `std`, with a plan-only first turn.
- Host: a set or unset that leaves a required key missing, or breaks a schema rule, is refused with the
  file unchanged.

## Searcher findings (search-1040, codex:luna:low, 45s)

- **Loading and writing.**
  - `internal/claude/repo_config.go:43` `LoadRepoConfig` validates the generic map against the embedded
    schema (`:75`, google/jsonschema-go), then decodes it into the typed `RepoConfig`.
  - `writeRepoConfig` marshals the struct, so comments and key order are lost. Its callers are
    `init.go:1159`, `init_wizard.go:124` and `repo_config.go:172`.
- **Keys** (`spec/schemas/repo.schema.json:5`), with their types:

  | Key | Type |
  |---|---|
  | `version` | const 1 |
  | `issues.tracker` | `harnez` or `none` |
  | `issues.merge_driver` | bool |
  | `issues.active_cap` | int, at least 1 |
  | `docs.studies` | bool |
  | `docs.copies` | []string |
  | `rules.quota` | bool |
  | `git.mode` | solo, fork or team |
  | `git.protected_main` | bool |
  | `git.default_branch` | string |
  | `make.scaffold` | bool |
  | `make.container_test` | bool |

  `additionalProperties` is false, so unknown keys are rejected. The schema also has the solo-mode rule
  (`if solo then protected_main false`) and declares no defaults.
- **Completion.** `ValidArgsFunction` is used by `issues` and the agent commands, and
  `RegisterFlagCompletionFunc` in `agent.go`/`agent_cmds.go`. Cobra provides `harnez completion fish`.
  No test calls `__complete` today.
- **No collision.** No `config` command exists, and the root commands are registered in
  `cmd/harnez/main.go:134`.
- **Keys that need `harnez init` after a change:** `issues.tracker`, `issues.merge_driver`,
  `docs.studies`, `docs.copies`, `rules.quota`, `git.mode`, `git.protected_main`, `make.scaffold` and
  `make.container_test`. `issues.active_cap` and `git.default_branch` don't.

## Milestones

- **M1 (core get/set/unset/list).** `harnez config` with `-d`, `--unset` and `--list`.
  - Keys and types come from the schema.
  - Values are parsed by type: bool, int, enum and string. An array key such as `docs.copies` takes a
    comma-separated value or a repeated `--add`; the Developer decides which in the plan.
  - Writes edit yaml.v3 nodes, so comments and order are kept.
  - The result is validated before writing, and the init hint is printed for render keys.
  - `--list` shows the effective values; keys left at the spec default (`spec/git.yaml`) are marked
    `(default)`.
- **M2 (completion and docs).**
  - Completion: `ValidArgsFunction` completes dotted keys first, then a key's enum values or
    `true`/`false`. Tests run through `__complete`.
  - Docs: the README command list, the help golden, and a short `docs/` mention of setting
    `git.mode`/`protected_main` this way.
