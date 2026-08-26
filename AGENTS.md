<!-- Keep this file token-efficient: use bullet lists, not tables; no redundant prose. -->
<!-- AGENTS.md is the canonical source; CLAUDE.md is a symlink to it. Edit AGENTS.md only. -->

<!-- harnez:begin Local Overlays -->
- Local ephemeral overrides: @AGENTS.local.md
<!-- harnez:end Local Overlays -->

<!-- harnez:begin Language Conventions -->
Adhere to the following conventions.

Docs in `./docs/` are managed by harnez. <!-- harnez:bundled -->

- Website Building Rules @docs/Website.md,
  no GitHub assumptions/octocats, match sibling website/ dirs or ask, honest/proven claims only,
  Why section required, relative links (subpage-hosted), static/no-CDN, opt-in JS demos only
- Go/Golang @docs/Go.md,
  Modern Go, avoid deps but use Cobra, add tests
- Bash/Shell @docs/Bash.md,
  Read before multi-line shell: Make recipes, embedded scripts
  No ";", break before then/else/docs
  No "if [[]]", No "if []", Use "if test"
  smart indent!
- Make/Makefile @docs/Make.md,
  ⚙️ phony sentinel, self-doc help, build dependency pattern
- Markdown @docs/Markdown.md,
  PascalCase for evergreens, kebab-case for ephemeral docs; ASCII art in chat, Mermaid only in docs/
- Git @docs/Git.md,
  conventional commits, work on the default branch, don't push unless asked
- Canary-first development @docs/Canary.md,
  probe external mechanisms before building features on them
- Spec system @docs/Spec.md,
  YAML spec files as single source of truth; Go code must not duplicate spec values
- Agentic Loop Practices @docs/AgenticLoop.md,
  5-phase loop (Advisory -> Dev -> Review -> Hygiene -> Retro), zero zombie guarantee
- Issue Tracking Practices @docs/IssueTracking.md,
  P0-P3 priorities, metadata headers (Status, Priority, Severity, Category), tracker sync
<!-- harnez:end Language Conventions -->

## CLI command scope

`apply` and `init` are intentionally separate — do not merge their concerns.

- `apply` — global `~/.claude` only: settings, hooks, commands, docs; flags: `-c`, `-t`, `-d`
- `init`  — project dir only: AGENTS.md, local sections, doc copies, Makefile; flags: `-c`, `-d`, `--docs`

Before changing any command's flags or adding project-local behaviour to `apply`, read
`docs/CLIDesign.md` — the separation is load-bearing and the footgun it prevents is real.

## Docs Layout

`docs/*.md` — this project's evergreen docs (architecture, decisions, pitfalls). Not copyable.

`docs/lang/` — copyable language/SDK/framework docs (Go, Bash, Make, Git, Rust, Cpp, Markdown, GTK4, Zig).
Installed to `~/.claude/docs/` on `apply`; copied into projects with `init --docs <name>`.

`docs/practices/` — copyable workflow and practice docs (AgenticLoop, IssueTracking). Same install mechanics as `docs/lang/`.

`docs/other/` — copyable docs that don't form a category yet (Canary, Spec). Same install mechanics as `docs/lang/`.

`docs/studies/` — case studies and background reports (reference material for future generic docs).

`docs/proposed/` — staging area for new docs that may become copyable. No install mechanics yet.

`docs/templates/` — Makefile scaffolding used by `init`; not docs.

Rule: if a doc applies to many projects → `docs/lang/`, `docs/practices/`, or `docs/other/`. If it describes this codebase → `docs/` root.
A category dir forms once 3+ docs share a theme.

## Issue Tracking & Priority Standards

Adhere to `@docs/IssueTracking.md` for issue tracking across `issues/*.md`:

- **Issue Priority Schema (Scheduling Urgency)**:
  - **P0 / Critical**: Blocker, data loss, security vulnerability, broken build, or critical regression violating core invariants. Halts regular development ("stop the line").
  - **P1 / High**: Core functionality broken, major workflow impediment, key API regression, or high-urgency milestone deliverable. Addressed in current sprint.
  - **P2 / Medium**: Normal feature, standard bug fix, performance optimization, UX polish, or refactoring without active blockage. Scheduled in normal backlog.
  - **P3 / Low**: Minor cosmetic glitch, typo, nice-to-have suggestion, speculative idea, or non-urgent documentation improvement. Opportunistic.
- **Priority vs. Severity**:
  - *Severity* = technical impact/damage (Critical, Major, Moderate, Minor).
  - *Priority* = scheduling urgency (P0, P1, P2, P3).
- **Ticket Metadata Format**: Every issue file under `issues/NNN-*.md` begins with:
  ```markdown
  # NNN — Title of Issue

  **Status**: Open | In Progress | Blocked — <reason> | Closed — resolved in <commit> | Draft
  **Priority**: P0 (Critical) | P1 (High) | P2 (Medium) | P3 (Low)
  **Severity**: Critical | Major | Moderate | Minor
  **Category**: Bug | Feature | Architecture | Documentation | Performance | Refactor | Agentic Ergonomics
  **Related**: [Doc / Ticket / Commit references]
  ```
- **Tracker Synchronization**: Keep `issues/README.md` table in sync with ticket files (`harnez status` verifies consistency). Archive closed tickets to `issues/archive/`.

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
- Regularly inspect spawned background tasks and explicitly terminate idle, completed, or zombie tasks.
- Clean up watch commands, poll loops, schedule timers, and background test subprocesses before finishing a task.
- Never abandon orphan processes or lingering watch tasks in the background.

## Context Discipline & Token Efficiency
- Do not execute whole-file read tools on `AGENTS.md`, `CLAUDE.md`, or rules already in the active system prompt.
- Prefer `grep_search` or range-bounded reads (`StartLine`/`EndLine`) over ingesting entire reference docs.
- Consult index annotations in `docs/README.md` to determine whether a bundled doc requires a full read or if its rule summary is sufficient.

## Voice & Transcription Input Awareness
- The user often uses voice-to-text / speech transcription (ASR).
- Be alert for phonetic homophones and transcription artifacts (e.g. "Southern Exploration" → "start an exploration agent", "harness" → "harnez"). Reason about user intent from phonetic similarity and conversation context before asking for clarification.

## Demo Recordings & Media Verification
- When creating, editing, or adding media assets (e.g. reels, WebM demos, screenshots) intended for documentation or websites, **always ask the user for explicit confirmation** that the recorded visual output matches their exact expectations before publishing or embedding it.

<!-- harnez:begin Repo Setup -->
## Repo Setup
- Solo/hobby repo — single default branch, no PR workflow.
- codeberg.org is primary; github.com (if present) is a synced mirror only.
<!-- harnez:end Repo Setup -->

