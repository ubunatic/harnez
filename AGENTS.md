<!-- Keep this file token-efficient: use bullet lists, not tables; no redundant prose. -->

<!-- claudeconfig:begin Language Conventions -->
Adhere to the following conventions.

- Go/Golang @docs/Go.md, 
  Modern Go, avoid deps but use Cobra, add tests
- Bash/Shell @docs/Bash.md,
  No ";", break before then/else/docs
  No "if [[]]", No "if []", Use "if test"
  smart indent!
<!-- claudeconfig:end Language Conventions -->

## Development Scripts

Run from project root.

- `scripts/smoke-test.sh` — build, apply, verify idempotency, simulate drift and confirm repair
- `scripts/drop-perm.sh PATTERN` — remove permissions matching PATTERN from `~/.claude/settings.json` for drift simulation
