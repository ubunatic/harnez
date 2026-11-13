**Once per session, before your first task, read all Harnez rules in one call: `harnez read .harnez/rules/Tools.md .harnez/rules/Issues.md .harnez/rules/Quota.md .harnez/rules/Subagents.md .harnez/rules/Output.md .harnez/rules/Local.md`. Absent optional files are skipped. Skip this if you already read them in this session. This follows Index.md order; Local.md overrides the other rules.**

<!-- Keep this file token-efficient: use bullet lists, not tables; no redundant prose. -->
<!-- AGENTS.md is the canonical source; CLAUDE.md is a symlink to it. Edit AGENTS.md only. -->

## Local Context Links

- Read local `@<file>` links immediately.
- Read local `See <file>` links before working on their corresponding tasks.
- Local `@<image>` and `See <image>` links are cheat sheets containing the full
  document content as token-efficient PNG cards; inspect them as documentation.

## CLI command scope

`apply` and `init` stay separate; do not merge their concerns.

- `apply`: global `~/.claude` only (settings, hooks, commands, docs); flags `-c`, `-t`, `-d`, `-s`.
- `init`: project dir only (AGENTS.md, local sections, doc copies, Makefile); flags `-c`, `-d`, `--docs`.
- Before changing a command's flags or adding project-local behaviour to `apply`, read `docs/CLIDesign.md`.
- New configurable values (endpoints, timeouts, model names, labels) go in `spec/` with a JSON
  schema, never as Go defaults; read `docs/Spec.md` first.

## Docs Layout

- `docs/*.md`: this project's evergreen docs. Not copyable.
- `docs/lang/`, `docs/practices/`, `docs/other/`: copyable docs (language, workflow, uncategorised).
  `apply` installs them to `~/.claude/docs/`; `init --docs <name>` copies them into a project.
- `docs/studies/`: case studies. `docs/proposed/`: staging for future copyable docs.
  `docs/templates/`: Makefile scaffolding for `init`.
- A doc that applies to many projects is copyable; one that describes this codebase goes in the
  `docs/` root. A category dir forms once 3+ docs share a theme.

## Rules and Copyable-Doc Sources

- Tool-neutral repo rules live in this file, outside the `harnez:begin/end` blocks. Harnez-only
  rules live in `.harnez/rules/`, generated from `config.yaml`; `Local.md` there is per-checkout
  and git-excluded.
- Copyable docs are the **source**. Their copies in `docs/*.md` (also in this repo),
  `~/.claude/docs/` and other projects follow only via `harnez init`/`apply`. Never edit a root
  copy alone: the next `harnez init` overwrites it.
- A root copy may come from a `*.lite.md` source (first line `<!-- harnez:variant=lite -->`);
  edit both the full and the lite source.
- After editing a source: `make install`, then `harnez init -d .` (init copies the docs embedded
  in the installed binary), `git checkout --` any unrelated file it rewrote, and commit source
  and copy together. `TestRootDocCopiesMatchSources` fails when they diverge.

## Development & Review

- Run from the project root. Run `make install` after changing Go code.
- Tickets follow `docs/IssueTracking.md`; sprints follow `docs/AgenticLoop.md` (`/sprint`).
  Before a review, check `docs/ReviewLearnings.md`.
- Small fixes: commit directly once all tests pass and existing assertions are intact. Larger
  changes and features: a fresh reviewer subagent checks test rigor, doc and ticket sync and
  code clarity before the commit.
- `scripts/smoke-test.sh`: build, apply, verify idempotency, simulate drift and confirm repair.
  `scripts/drop-perm.sh PATTERN`: remove matching permissions from `~/.claude/settings.json`
  for drift simulation.
- `gitOutput` (`internal/issues/maintenance`) trims its result and eats the leading space of
  porcelain lines like ` M path`. Never feed it to a line-based parser; use `porcelainStatus`
  for `git status --porcelain`. It is fine for single values (`rev-parse`) and emptiness checks.
- Logs: after one or two targeted grep/tail checks without a root cause, stop reading and reason
  from first principles or ask. Never load raw logs or whole transcripts into context.

## Subagents

- A handoff must not block the main chat: delegate, stay responsive, and wait only when the user
  asks or the next step needs the result.
- No git worktree isolation unless the user asks for it. Sequential ticket work runs on the
  checked-out branch; worktrees branch from stale bases and cannot see a prior step's
  uncommitted changes.

## Voice Input

Speech-to-text homophones in this project: "Southern Exploration" means "start an exploration
agent"; "harness" means "harnez".

<!-- harnez:begin Repo Setup -->
## Repo Setup
- Team repo: work on a local topic branch and open a PR to merge. Tickets and small PRs on a forge with AGit (Forgejo, Gitea) reuse one local branch and push no branch: `git push origin HEAD:refs/for/<default-branch>/<topic>`.
- Push a branch (`issue/<n>-<slug>`, `sprint/<n>`) for work others push to, long sprints that need rebasing, and GitHub, which has no AGit.
- main is protected; harnez refuses commits on it, except `harnez release`, which the repository admin or an agent asked to release may run there. Otherwise do not commit to or push the default branch unless the repository admin explicitly authorizes it in the current session. Never force-push.
- Update/rebase your branch before merging; follow this repo's CI and review requirements.
<!-- harnez:end Repo Setup -->

## Remotes
- Codeberg and GitHub are both dev remotes; Codeberg is canonical for releases (`docs/Git.md`, Remotes).
- `origin` is `ubunatic/harnez` on Codeberg with a protected `main`: every change except
  `harnez release` goes through a topic branch and a PR, which the host opens.
- Owner's clone with the `both` remote: `make install-git-hooks` copies `scripts/git-hooks/pre-push`
  (Git LFS plus the GitHub guard) into the hooks dir. It refuses to replace a differing hook that
  lacks the `git lfs pre-push` line unless `FORCE=1`. Tests: `internal/githooks`.

<!-- harnez:begin Language Conventions -->
Adhere to the following conventions.

Docs in `./docs/` are managed by harnez. <!-- harnez:bundled -->

- Agentic Loop Practices @docs/AgenticLoop.md,
  5-phase loop (Advisory -> Dev -> Review -> Hygiene -> Retro), zero zombie guarantee
- AGit @docs/AGit.md,
  this repo uses AGit: push HEAD:refs/for/<default>/<topic>, no branch; merged topic opens a new PR; harnez pr wait for the merge
- Bash/Shell @docs/Bash.md,
  Read before multi-line shell: Make recipes, embedded scripts
  No ";", break before then/else/docs
  No "if [[]]", No "if []", Use "if test"
  smart indent!
  Use git -C/make -C, not cd
- Canary-first development @docs/Canary.md,
  probe external mechanisms before building features on them
- ConciseMode Practices @docs/ConciseMode.md,
  3 graded terseness tiers (Lite, Standard, Ultra); code/diffs/commands always verbatim
- Containerfile build efficiency @docs/Containerfile.md,
  order layers static-first/dynamic-last; git+git-lfs mandatory in agent-facing images
- Git @docs/Git.md,
  conventional commits, work on the default branch, don't push unless asked
- Go/Golang @docs/Go.md,
  Modern Go; substantive features and tests in internal/<feature>, thin cmd/<app> for test caching; avoid deps, use runes and display width
- Release Pipeline @docs/GoRelease.md,
  harnez release, version.yaml spec, GoReleaser v2, non-interactive minisign (-W), Forgejo has_releases, language-agnostic (Go/Python/Zig/Rust/scripted)
- Issue Tracking Practices @docs/IssueTracking.md,
  P0-P3 priorities, metadata headers (Status, Priority, Severity, Category), tracker sync
- Make/Makefile @docs/Make.md,
  ⚙️ phony sentinel, self-doc help, build dependency pattern
- Man Pages for Go CLIs @docs/ManPages.md,
  cobra/doc GenManTree; <cmd> man / man --install; XDG ~/.local/share/man/man1
- Markdown @docs/Markdown.md,
  PascalCase for evergreens, kebab-case for ephemeral docs; ASCII art in chat, Mermaid only in docs/
- Feature prototyping @docs/PrototypingFeatures.md,
  canary-first is the feature-prototyping practice; separate runtime isolation and rollout flags
- Search Practices @docs/Search.md,
  harnez find code/docs, finder configuration, partial results, and rg fallback
- Spec system @docs/Spec.md,
  YAML spec files as single source of truth; Go code must not duplicate spec values
- Test Tiers @docs/TestTiers.md,
  make lint / test-pkg / test-short / test / container-test; no sleeps, raceFactor budgets, PTY tests in the full tier
- Website Building Rules @docs/Website.md,
  no GitHub assumptions/octocats, match sibling website/ dirs or ask, honest/proven claims only,
  Why section required, relative links (subpage-hosted), static/no-CDN, opt-in JS demos only
<!-- harnez:end Language Conventions -->
