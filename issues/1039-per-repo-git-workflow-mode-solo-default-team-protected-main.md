# 1039 — Per-repo git workflow mode: solo (default), team, protected main

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Enhancement
**Related**: [[864]] (session push consent, release pushes per remote), [[836]], docs/Git.md, docs/GoRelease.md

---

## Goal

/goal Make the git workflow a single per-repo setting in `.harnez/repo.yaml`. The default stays `solo`;
`team` and protected-main modes make the agent rules, sprint skills and harnez's own commits and pushes
follow the branch + PR flow. Verify each mode in a scratch repo. Stop and report when blocked on a user
decision or a denied permission. On a loop, a hard blocker, or no progress, clear the goal and write a
failure report here.

## 1. Problem & Motivation

harnez assumes solo work: `docs/Git.md` says to work on the default branch, and `harnez issues new|close`
and the sprint skills commit straight to `main`. harnez itself is now worked on as a team.
`ubunatic/harnez` is the canonical remote (it is `origin` again; the old `flashcoder` copy is a remote
that only the trigger or the owner pushes), and its `main` is protected. Every change must go through a branch
and a PR (e.g. PR #14 for 1036/1037). Today that is handled by hand on every session, and nothing in the repo
records it, so the next agent may commit tickets to `main` and try to push.

## 2. Proposal

A per-repo key, e.g. `git.mode` in `.harnez/repo.yaml` (name open), with `harnez init` rendering the matching rules:

| Mode | Commits | Publishing |
|---|---|---|
| `solo` (default, today) | on `main` | push `main` when asked |
| `team` | on a topic branch per ticket/sprint | push the branch, open a PR (`tea`/`gh` per forge) |
| protected `main` | as team; never commit to local `main` | as team; never push `main` or force-push |

- Setting it must be one command or one line, e.g. `harnez init --git-mode team`.
- Rendered guidance (docs/Git.md copy, rules) and the sprint/issue skills read the mode instead of assuming `main`.
- `harnez issues new|close` and other auto-committing commands: in team/protected modes, refuse on `main`
  or commit to the current topic branch.
- `harnez release`: check that tag and branch pushes work with a protected `main`.

## 3. Open Questions

- Is protected `main` its own mode, or a flag on `team` (`git.protected_main: true`)? Could it be read
  from the forge API instead?
- Branch naming (`issue/<n>-<slug>`, `sprint/<tickets>`) and who opens the PR: host or developer.
- Where ticket-only commits go in team mode: their own PR, or riding along with the next code PR.

## 4. Verification

- Scratch repo per mode: `harnez init` renders the matching rules, and the issue commands commit where the mode says.
- In this repo, `team` plus protected `main` is set, and a full lean sprint ends in a PR with no push to `main`.

## Decision

- 2026-10-30, owner: an Architect was asked for before development (arch-1039, claude:opus, 1m03s).
- Host: the Architect's design below is adopted as the plan, including its changes to the host proposal: a leaf package
  `internal/gitflow`; team without protection warns instead of refusing; release refuses before the bump.
  Cuts: no forge API, no PR creation, no tag-after-merge, no automatic branch switching.
- Host: 1039 is delivered on branch `sprint/1039` and published as a PR, since origin `main` is protected.
- Host (dev plan review): all 8 developer corrections were adopted. In short:
  - `ProtectedBranches(dir, c)` helper; `LoadRepoConfig` validates gitflow first; an absent git key stays zero.
  - The ApplyStatus guard goes after the no-op checks; mv/rebase/reconcile are guarded up front, with gitAddAndCommit as a backstop.
  - `index --stage` is not guarded; warnings go through `gitflow.Stderr`; the release push refusal applies only with protected_main.
  - M4 validates before writing; M5 edits the sources in docs/commands and regenerates the marker blocks with init.
- Host: release dry-run runs the guard too, so the preview matches reality.
- Host: two review gates: M1-M2 (schema, loader, commit guard), then M3-M5 (release, init, docs).

## Architecture (arch-1039, adopted)

### 1. Schema, types, defaults, default branch

```yaml
# .harnez/repo.yaml
git:
  mode: team            # solo | fork | team; default solo (key absent = solo)
  protected_main: true  # default false; schema rejects true when mode is solo
  default_branch: main  # optional override; otherwise detected
```

Schema: add `git` as an optional object with `additionalProperties: false`, `mode` as an enum, `protected_main` as a boolean, and `default_branch` as a string with `minLength: 1`. Add an `if {mode: const solo}` / `then {protected_main: const false}` rule. `git` stays out of the top-level `required`, so existing repo.yaml files still validate.

The defaults (`mode: solo`, `protected_main: false`) and the fallback branch names (`[main, master]`) go in a new `spec/git.yaml` with its own schema. They are not Go literals.

Go: new package `internal/gitflow`. It imports only the root `harnez` package (for `DefaultFS`) and yaml.

```go
type Mode string // "solo" | "fork" | "team"
type Config struct {
    Mode          Mode   `yaml:"mode"`
    ProtectedMain bool   `yaml:"protected_main"`
    DefaultBranch string `yaml:"default_branch,omitempty"`
}
func Load(dir string) (Config, error)                   // reads only repo.yaml's git key; missing file or key = spec defaults
func DefaultBranch(dir string, c Config) (string, bool) // see order below
func GuardCommit(dir, action string) error              // the one shared helper
```

`claude.RepoConfig` gets a field `Git gitflow.Config \`yaml:"git"\``. Full schema validation stays in `LoadRepoConfig`. `gitflow.Load` also checks the enum itself, so an invalid value still fails on code paths that never call `LoadRepoConfig`.

**How the default branch is found, in order:**

1. `git.default_branch` from repo.yaml.
2. `git symbolic-ref --short refs/remotes/origin/HEAD`, with the `origin/` prefix stripped.
3. No origin HEAD: treat every name in the spec fallback list (`main`, `master`) as protected, and say so in the error. This is the conservative choice.

### 2. Behaviour by mode and protected_main

`GuardCommit(dir, action)` runs before every `git add`/`git commit` in harnez:

| state | solo | team or fork, unprotected | team or fork + `protected_main` |
|---|---|---|---|
| on a topic branch | commit | commit | commit |
| on the default branch | commit | commit, plus a stderr warning: `git.mode=team: committing on main; use a topic branch` | **refuse**: `git.protected_main is set in .harnez/repo.yaml: refusing to <action> on 'main'; run: git switch -c issue/<n>-<slug>` |
| detached HEAD | commit (today's behaviour) | warn | **refuse**: `detached HEAD; check out a branch` |
| bad `repo.yaml` / unknown mode | error naming the file and key | same | same |

Every commit path calls it:

- **Issue status verbs:** `internal/issues/status_action.go:71`, before the commit step.
- **`issues mv`:** `cmd/harnez/issues.go:942` inside `gitAddAndCommit`. That one function covers 696, 857 and the status-verb CLI path.
- **`issues rebase`:** `maintenance/rebase.go:439`. Rebase already refuses detached HEAD at `:170`; keep that.
- **`issues reconcile`:** `maintenance/reconcile.go:190`.
- **`--check` and `--no-commit`:** these skip the guard, because they don't commit.

**Release** (`runner.go`):

- With protected main, the release checks `GuardCommit(dir, "release")` *before the version bump writes any file*, so a refused release leaves nothing half-done.
- On a topic branch it bumps, commits and tags as usual. Then `pushRelease` pushes the branch and the tag; the tag points at the branch commit.
- The documented flow is: a `release/vX.Y.Z` branch, a PR, then merge. A merge commit or squash merge leaves the tag off `main`'s history. The doc must say "rebase-merge or fast-forward only", or the release takes the tag from the merged `main` with `--continue`.
- **Cut for now:** I would not build tag-after-merge support. Document it instead.
- `--skip-push` still works.
- The push check before the first push (multi-remote, issue 864) is: if the current branch equals the default branch, refuse, and do this before looping over remotes. That makes the partial push described in the ticket impossible.

**`init` rendering:**

- `--repo-mode X` (and the new alias `--git-mode`) writes `git.mode`. `--protected-main` writes `git.protected_main`. With no flag, init renders from repo.yaml.
- `repo_modes` gains a `team_protected` content variant. It is chosen when `protected_main` is true. `fork` + protected reuses the fork text plus the protected line.
- The `.harnez/rules/Index.md` fallback mechanism (`init.go:505-529`) adds a short `Git workflow: <mode>[, protected main]` line, so every agent sees it.

### 3. Milestones

**M1: schema, spec and loader** (write set: `spec/git.yaml`, `spec/schemas/git.schema.json`, `spec/schemas/repo.schema.json`, `internal/gitflow/{gitflow.go,gitflow_test.go}`, `internal/claude/repo_config.go` + test)

- `TestLoadDefaultsWhenGitKeyAbsent` → solo, false.
- `TestLoadTeamProtected` → team, true.
- `TestSchemaRejectsUnknownMode` (`mode: trunk`) → error naming `git.mode`.
- `TestSchemaRejectsProtectedOnSolo` (solo + true) → validation error.
- `TestSchemaRejectsUnknownGitKey` (`git.foo`) → error.
- `TestDefaultBranchOverride` (`default_branch: trunk`) → trunk.
- `TestDefaultBranchFromOriginHEAD` (scratch repo, bare origin with HEAD → `dev`) → dev.
- `TestDefaultBranchNoOriginHEAD` → ok=false, protected set is {main, master}.

**M2: `GuardCommit` plus wiring the issue commands** (write set: `gitflow.go`, `status_action.go`, `cmd/harnez/issues.go`, `rebase.go`, `reconcile.go`, their tests)

- `TestGuardSoloOnMain` → nil.
- `TestGuardTeamOnMainWarns` → nil plus the warning on stderr.
- `TestGuardProtectedOnMainRefuses` → error that contains both `git.protected_main` and `git switch -c`.
- `TestGuardProtectedOnTopicBranch` → nil.
- `TestGuardProtectedDetachedHEAD` → refuse.
- `TestGuardProtectedNoOriginHEADOnMaster` → refuse.
- `TestIssuesCloseRefusedOnProtectedMain` → the ticket file is unchanged and no new commit exists. That means the guard must run before any write, so call it first in `ApplyStatus`.
- `TestIssuesCloseCommitsOnTopicBranch` → HEAD advances on the branch.
- `TestIssuesCheckSkipsGuard` → `--check` passes on protected main.

**M3: release** (write set: `internal/release/runner.go`, `push.go`, tests)

- `TestReleaseRefusesOnProtectedMainBeforeBump` → `version.yaml` is untouched, there is no tag, and no push is attempted to the fake remote.
- `TestReleaseOnBranchPushesBranchAndTag` → the bare remote has the branch and the tag, and `main` is unchanged.
- `TestReleaseSoloUnchanged` → the existing golden output still matches.

**M4: `init` persistence and rendering** (write set: `cmd/harnez/init.go`, `internal/claude/init_phases.go`, `init.go`, `config.yaml` `repo_modes`, `testdata` golden AGENTS/Index files)

- `TestInitRepoModeWritesRepoYAML` (`--repo-mode team --protected-main`) → repo.yaml has `git.mode: team` and `protected_main: true`.
- `TestInitRendersFromRepoYAMLWithoutFlag` → team_protected section in AGENTS.md.
- `TestInitProtectedMainOnSoloRejected` → flag error, nothing written.
- `TestInitSoloGoldenUnchanged`.

**M5: docs and skills** (wording in section 4)

- Then sync the root copies with `harnez init -d .` and `git checkout --` any unrelated file it rewrites.
- `TestRootDocCopiesMatchSources` must pass.
- Set this repo's `.harnez/repo.yaml` to team + protected. That is the ticket's own check.

### 4. Doc and skill wording

- **`docs/lang/Git.md` (and any lite source), then the `docs/Git.md` copy:** "Workflow follows `git.mode` in `.harnez/repo.yaml`. solo: work on the default branch. team/fork: one topic branch per ticket or sprint (`issue/<n>-<slug>`, `sprint/<n>`), push the branch, open a PR. protected_main: never commit to or push the default branch, never force-push."
- **`docs/GoRelease.md` (source plus copy):** under protected main, release from `release/vX.Y.Z`, then rebase-merge the PR so the tag stays on `main`'s history.
- **`lean-sprint.md` (lines 74-75, 85, 90-91), `sprint.md:58`, `factory-sprint.md:89-93`:** "Before Dev, read `git.mode`. If it isn't solo, create or reuse `sprint/<tickets>`. The final step pushes the branch and reports the PR URL (the host opens it with tea/gh), instead of committing to main."
- **`issue.md` (lines 28, 86-93):** "In team/protected mode, the ticket commit rides on the current topic branch. On the default branch, create `issue/<n>-<slug>` first."
- **`release.md:13-16`:** "With protected_main, branch, then release, then PR. Never push main."
- **`config.yaml` `repo_modes`:**
  - `team`: "topic branch per ticket, PR to merge".
  - `team_protected`: adds "main is protected; harnez refuses commits on it".
  - `solo`: unchanged.
- **`CLAUDE.md` / AGENTS.md "Repo Setup":** this repo's text is regenerated as team + protected. The "Solo/hobby repo" line is now wrong for this repo.

### 5. Risks and cuts

- **Ticket commits on a branch drift out of `main`'s index.** If two branches each close tickets, there will be index conflicts. The merge driver (`issues.merge_driver`) helps; test it in M5.
- **Detecting the default branch from origin HEAD** is stale after a forge-side rename. `default_branch` is the way out.
- **The existing team text already says "protected branches".** Now that `protected_main` exists separately, M4 must reword it so team doesn't imply protection.
- **Cut:**
  - Reading branch protection from the forge API.
  - Creating PRs (out of scope, as you proposed).
  - Tag-after-merge release support.
  - Making `GuardCommit` switch to a new branch automatically. Refusing with the exact command is enough and safer.
- **Kept separate on purpose:** `protected_main` instead of a fourth mode. That way fork + protected works without a combined mode for every pairing.
