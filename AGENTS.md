<!-- Keep this file token-efficient: use bullet lists, not tables; no redundant prose. -->
<!-- AGENTS.md is the canonical source; CLAUDE.md is a symlink to it. Edit AGENTS.md only. -->

<!-- claudeconfig:begin Language Conventions -->
Adhere to the following conventions.

- Go/Golang @docs/Go.md,
  Modern Go, avoid deps but use Cobra, add tests
- Bash/Shell @docs/Bash.md,
  No ";", break before then/else/docs
  No "if [[]]", No "if []", Use "if test"
  smart indent!
- Make/Makefile @docs/Make.md,
  ⚙️ phony sentinel, self-doc help, build dependency pattern
- Git @docs/Git.md,
  conventional commits, work on the default branch, don't push unless asked
<!-- claudeconfig:end Language Conventions -->

## CLI command scope

`apply` and `init` are intentionally separate — do not merge their concerns.

- `apply` — global `~/.claude` only: settings, hooks, commands, lang docs; flags: `-c`, `-t`, `-l`
- `init`  — project dir only: AGENTS.md, local sections, lang doc copies, Makefile; flags: `-c`, `-d`, `-l`

Before changing any command's flags or adding project-local behaviour to `apply`, read
`docs/CLIDesign.md` — the separation is load-bearing and the footgun it prevents is real.

## Development Scripts

Run from project root.

- `scripts/smoke-test.sh` — build, apply, verify idempotency, simulate drift and confirm repair
- `scripts/drop-perm.sh PATTERN` — remove permissions matching PATTERN from `~/.claude/settings.json` for drift simulation
<!-- claudeconfig:begin Repo Setup -->
## Repo Setup
- Solo/hobby repo — single default branch, no PR workflow.
- codeberg.org is primary; github.com (if present) is a synced mirror only.
<!-- claudeconfig:end Repo Setup -->
