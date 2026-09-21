<!-- Keep this file token-efficient: use bullet lists, not tables; no redundant prose. -->
<!-- AGENTS.md is the canonical source; CLAUDE.md is a symlink to it. Edit AGENTS.md only. -->

## Local Context Links

- Read local `@<file>` links immediately.
- Read local `See <file>` links before working on their corresponding tasks.
- Local `@<image>` and `See <image>` links are cheat sheets containing the full
  document content as token-efficient PNG cards; inspect them as documentation.

<!-- harnez:begin Local Overlays -->
- Local ephemeral overrides: @AGENTS.local.md
<!-- harnez:end Local Overlays -->

<!-- harnez:begin Harnez Managed Conventions -->
## Harnez Managed Conventions

Managed by harnez — local edits here are overwritten on the next `harnez init`.
Put project-specific rules outside this block.

### Editing Discipline
- Prefer structured patch tools (`apply_patch`) or whole-block replacements over
  narrow string substitution edits.
- When making multi-line edits, ensure sufficient surrounding context lines to
  avoid ambiguous pattern matches.
- **Reading & Context Discipline (Recommended for Large Files)**: Prefer
  `harnez read -I <file>` (dense visual PNG context card) or
  `harnez read -L <range>` / `harnez read -n` for medium/large files (>100 lines)
  to preserve token quota and prevent context fatigue. Native reads remain valid
  for targeted inspection; hook-level blocking is conditional on the active
  `reading_discipline.enforce` mode in `~/.harnez/config.yaml` (or
  `HARNEZ_READ_ENFORCE`).

### Issue Tracker Discovery (harnez find)
Applies when this project has an `issues/` tracker. To search existing issues,
compute the next ticket number, or allocate one, use `harnez find` / `harnez issues`
instead of `ls issues/`, `find`, or raw grep:
- `harnez find -d <repo> issues -a status:open` — list active open issues (use `-I` for visual overview PNG card)
- `harnez find -d <repo> issues "<query>"` — fuzzy search across titles and body text (use `-I` for visual overview)
- `harnez issues show -d <repo> <n> -I` — render single issue as styled visual PNG card (inspect via `view_file`)
- `harnez find -d <repo> issues next` — report the next free ticket number (read-only)
- `harnez issues new -d <repo> "<title>"` — atomically reserve that number and create
  a placeholder ticket file; write the ticket to the printed path
- `harnez issues <verb> -d <repo> <n> [reason]` — change a ticket's status, resync
  `issues/README.md`, and commit, in one call
- `harnez index -d <repo>` — update `issues/README.md` after filing or updating tickets
- Commit documentation and `issues/*.md` changes immediately; don't batch them behind
  pending code work.

### Agentic Loop Invariants
Where `@docs/AgenticLoop.md` is present in this project, follow it rather than
restating it here — in particular Invariant 1 (Parallel Read, Sequential Write:
one writer per workspace), Invariant 3 (Zero Zombie Guarantee: track and terminate
every background task and subagent), Invariant 6 (Context Discipline: no whole-file
reads of AGENTS.md/CLAUDE.md — grep or range-bounded reads), and Invariant 10
(Media & Demo Verification Gate: explicit user confirmation before publishing
recordings or screenshots).
<!-- harnez:end Harnez Managed Conventions -->

## CLI command scope

`apply` and `init` are intentionally separate — do not merge their concerns.

- `apply` — global `~/.claude` only: settings, hooks, commands, docs; flags: `-c`, `-t`, `-d`, `-s`
- `init`  — project dir only: AGENTS.md, local sections, doc copies, Makefile; flags: `-c`, `-d`, `--docs`

Before changing any command's flags or adding project-local behaviour to `apply`, read
`docs/CLIDesign.md` — the separation is load-bearing and the footgun it prevents is real.

## Docs Layout

`docs/*.md` — this project's evergreen docs (architecture, decisions, pitfalls). Not copyable.

`docs/lang/` — copyable language/SDK/framework docs (Go, Bash, Make, Git, Rust, Cpp, Markdown, GTK4, Zig).
Installed to `~/.claude/docs/` on `apply`; copied into projects with `init --docs <name>`.

`docs/practices/` — copyable workflow and practice docs (AgenticLoop, IssueTracking, PrototypingFeatures). Same install mechanics as `docs/lang/`.

`docs/other/` — copyable docs that don't form a category yet (Canary, Spec, Containerfile). Same install mechanics as `docs/lang/`.

`docs/studies/` — case studies and background reports (reference material for future generic docs).

`docs/proposed/` — staging area for new docs that may become copyable. No install mechanics yet.

`docs/templates/` — Makefile scaffolding used by `init`; not docs.

Rule: if a doc applies to many projects → `docs/lang/`, `docs/practices/`, or `docs/other/`. If it describes this codebase → `docs/` root.
A category dir forms once 3+ docs share a theme.

## Where Repo Rules Go, and Copyable-Doc Sources

- Repo-specific rules live in this file, **outside** the `harnez:begin/end` blocks, which
  `harnez init` overwrites. `AGENTS.local.md` is git-excluded and only for ephemeral local
  overrides; never put durable rules there.
- Copyable docs (`docs/practices/`, `docs/lang/`, `docs/other/`) are the **source**. Their copies
  in `docs/*.md` (also in this repo), `~/.claude/docs/` and other projects only follow via
  `harnez init`/`apply`. Edit the source file, never the root copy alone: a root-only edit is
  silently overwritten by the next `harnez init`. After editing a source, sync the root copy
  (`harnez init -d .`, then `git checkout --` any unrelated file it rewrites) and commit both.

## Issue Tracking & Priority Standards

Adhere to `@docs/IssueTracking.md` for issue tracking conventions across `issues/*.md`
(P0-P3 priority schema, severity/priority distinction, ticket metadata header format,
tracker synchronization).

## Development & Review Workflow

Run from project root.

- Follow the 5-phase sprint workflow (`@docs/AgenticLoop.md` / `/sprint`):
  1. Parallel Advisory Discovery (read-only audits)
  2. Sequential Development & TDD
  3. Pre-Commit Review Gate (independent reviewer subagent)
  4. Process & Subagent Hygiene (drain/kill background tasks and timers)
  5. Flow Quality Retrospective (`docs/feedback/` or `docs/studies/`)
- Always run `make install` after modifying Go code to update the local binary in `~/go/bin`.
- `scripts/smoke-test.sh` — build, apply, verify idempotency, simulate drift and confirm repair
- `scripts/drop-perm.sh PATTERN` — remove permissions matching PATTERN from `~/.claude/settings.json` for drift simulation
- Small fixes: direct commit is permitted once all tests pass and existing test assertions remain intact.
- Larger changes & features: require a review pass before commit (orchestrated across subagents or by spawning a fresh reviewer subagent if acting as main agent) to verify test rigor, doc/ticket sync, and code clarity for future agents.

## Troubleshooting & Log Exploration
- Do not get trapped exhaustively browsing transcript/system logs.
- If a root cause is not apparent after 1–2 targeted grep/tail inspections, stop reading logs, reason from first principles, or ask for guidance.
- Never ingest large raw logs or whole transcript files into context.

## Background Tasks & Process Hygiene
See `@docs/AgenticLoop.md` Invariant 3 (Zero Zombie Guarantee) for tracking and
terminating background tasks, watch loops, and subagents.
- Subagent handoff must not block the main chat. When the user asks to hand work to a subagent,
  spawn/delegate the task and remain responsive as the host orchestrator; do not immediately wait
  on the child agent unless the user explicitly asks you to wait or the next user-visible
  integration step truly cannot proceed without the result.
- Do not spawn subagents with git worktree isolation unless the user explicitly requests it.
  Sequential/consecutive ticket work should run directly on the currently checked-out branch —
  worktrees have their own failure modes (e.g. branching from a stale base, or being unable to
  see a prior step's still-uncommitted changes) and add reconciliation overhead that isn't needed
  for normal one-after-another dev work.

## Context Discipline & Token Efficiency

See `@docs/AgenticLoop.md` Invariant 6 (Context Discipline & Range-Bounded Ingestion)
for the canonical statement of this rule.

## Voice & Transcription Input Awareness
Global Voice Input rule applies (`~/.claude/CLAUDE.md`); project-specific homophone
example: "Southern Exploration" → "start an exploration agent", "harness" → "harnez".

## Demo Recordings & Media Verification
See `@docs/AgenticLoop.md` Invariant 10 (Media & Demo Verification Gate).

<!-- harnez:begin Repo Setup -->
## Repo Setup
- Solo/hobby repo — single default branch, no PR workflow.
- codeberg.org is primary; github.com (if present) is a synced mirror only.
<!-- harnez:end Repo Setup -->
<!-- harnez:begin Language Conventions -->
Adhere to the following conventions.

Docs in `./docs/` are managed by harnez. <!-- harnez:bundled -->

- Issue Tracking Practices @docs/IssueTracking.md,
  P0-P3 priorities, metadata headers (Status, Priority, Severity, Category), tracker sync
- Agentic Loop Practices @docs/AgenticLoop.md,
  5-phase loop (Advisory -> Dev -> Review -> Hygiene -> Retro), zero zombie guarantee
- Bash/Shell @docs/Bash.md,
  Read before multi-line shell: Make recipes, embedded scripts
  No ";", break before then/else/docs
  No "if [[]]", No "if []", Use "if test"
  smart indent!
  Use git -C/make -C, not cd
- Canary-first development @docs/Canary.md,
  probe external mechanisms before building features on them
- Git @docs/Git.md,
  conventional commits, work on the default branch, don't push unless asked
- Go/Golang @docs/Go.md,
  Modern Go, avoid deps but use Cobra, add tests; use runes and display width for terminal layout
- Release Pipeline @docs/GoRelease.md,
  harnez release, version.yaml spec, GoReleaser v2, non-interactive minisign (-W), Forgejo has_releases, language-agnostic (Go/Python/Zig/Rust/scripted)
- Make/Makefile @docs/Make.md,
  ⚙️ phony sentinel, self-doc help, build dependency pattern
- Man Pages for Go CLIs @docs/ManPages.md,
  cobra/doc GenManTree; <cmd> man / man --install; XDG ~/.local/share/man/man1
- Markdown @docs/Markdown.md,
  PascalCase for evergreens, kebab-case for ephemeral docs; ASCII art in chat, Mermaid only in docs/
- Feature prototyping @docs/PrototypingFeatures.md,
  canary-first is the feature-prototyping practice; separate runtime isolation and rollout flags
- Spec system @docs/Spec.md,
  YAML spec files as single source of truth; Go code must not duplicate spec values
<!-- harnez:end Language Conventions -->
<!-- harnez:begin Quota-1 Guardrails -->
## Quota-1 Guardrails

- **Single-Test Boundary**: Under Quota-1 rules, the agent may only run the test suite once per step/turn.
- **Code Modification Required**: If tests fail or complete, you MUST modify repository source files before running tests again. Repeated test runs without intermediate code modifications are blocked.
- **Enforced Test Target**: Execute tests via `make test-q1` (or `harnez exec --quota-1 -- <test-cmd>`).
- **Unauthorized Bypass Forbidden**: Bypassing guardrails via `QUOTA_BYPASS=1` or `HARNEZ_QUOTA_BYPASS=1` is strictly reserved for human developers and CI environments. Agent loops must not set or pass bypass flags.
<!-- harnez:end Quota-1 Guardrails -->
