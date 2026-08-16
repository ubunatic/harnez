<!-- Keep this file token-efficient: use bullet lists, not tables; no redundant prose. -->
<!-- AGENTS.md is the canonical source; CLAUDE.md is a symlink to it. Edit AGENTS.md only. -->

<!-- claudeconfig:begin Language Conventions -->
Adhere to the following conventions.

Docs in `./docs/` are managed by claudeconfig. <!-- claudeconfig:bundled -->

- Go/Golang @docs/Go.md,
  Modern Go, avoid deps but use Cobra, add tests
- Bash/Shell @docs/Bash.md,
  No ";", break before then/else/docs
  No "if [[]]", No "if []", Use "if test"
  smart indent!
- Make/Makefile @docs/Make.md,
  ⚙️ phony sentinel, self-doc help, build dependency pattern
- Markdown @docs/Markdown.md,
  PascalCase for evergreens, kebab-case for ephemeral docs
- Git @docs/Git.md,
  conventional commits, work on the default branch, don't push unless asked
- Canary-first development @docs/Canary.md,
  probe external mechanisms before building features on them
- Spec system @docs/Spec.md,
  YAML spec files as single source of truth; Go code must not duplicate spec values
<!-- claudeconfig:end Language Conventions -->

## CLI command scope

`apply` and `init` are intentionally separate — do not merge their concerns.

- `apply` — global `~/.claude` only: settings, hooks, commands, docs; flags: `-c`, `-t`, `-d`
- `init`  — project dir only: AGENTS.md, local sections, doc copies, Makefile; flags: `-c`, `-d`, `--doc`

Before changing any command's flags or adding project-local behaviour to `apply`, read
`docs/CLIDesign.md` — the separation is load-bearing and the footgun it prevents is real.

## Docs Layout

`docs/*.md` — this project's evergreen docs (architecture, decisions, pitfalls). Not copyable.

`docs/lang/` — copyable language/SDK/framework docs (Go, Bash, Make, Git, Rust, Cpp, Markdown, GTK4, Zig).
Installed to `~/.claude/docs/` on `apply`; copied into projects with `init --doc <name>`.

`docs/other/` — copyable docs that don't form a category yet (Canary, Spec). Same install mechanics as `docs/lang/`.

`docs/studies/` — case studies and background reports (reference material for future generic docs).

`docs/proposed/` — staging area for new docs that may become copyable. No install mechanics yet.

`docs/templates/` — Makefile scaffolding used by `init`; not docs.

Rule: if a doc applies to many projects → `docs/lang/` or `docs/other/`. If it describes this codebase → `docs/` root.
A category dir (e.g. `docs/practices/`) forms once 3+ docs share a theme.

## Development Scripts

Run from project root.

- `scripts/smoke-test.sh` — build, apply, verify idempotency, simulate drift and confirm repair
- `scripts/drop-perm.sh PATTERN` — remove permissions matching PATTERN from `~/.claude/settings.json` for drift simulation
<!-- claudeconfig:begin Repo Setup -->
## Repo Setup
- Solo/hobby repo — single default branch, no PR workflow.
- codeberg.org is primary; github.com (if present) is a synced mirror only.
<!-- claudeconfig:end Repo Setup -->
