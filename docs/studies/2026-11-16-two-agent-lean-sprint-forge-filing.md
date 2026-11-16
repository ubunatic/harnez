---
kind: sprint
---
# Lean sprint 1228, 1212, 1202: forge filing through harnez issues (2026-11-16)

Goal: filing on a forge tracker through `harnez issues` works for create, edit, comment, close
and reopen. Branch `sprint/1228-1212-1202`.

## Agents

- Searcher `forge-scout`, codex:luna:low: round 1 (verb map) 1m14s, 60.7k new tokens.

## Searcher round 1: verb map on a forge tracker

Questions: what each `harnez issues` verb does on a forge tracker, where `--host` is added to
`fj`, which `fj`/`tea`/`gh` forms exist, how the comment body is checked, how tests fake the CLIs.

- `new`: forge path exists (`internal/forgetracker/create.go:50`); with fj it builds
  `fj --host <host> issue create --repo <slug> --body-file <file> <title>`.
- `show`: forge path exists and uses harnez's own API client, no external CLI
  (`internal/forgetracker/forgetracker.go:88`).
- `comment`: forge path exists (`cmd/harnez/issues_forge.go:126`); it builds its own
  `fj --host <host> issue comment <slug>#N --body-file <file>` (`:161`), so it has the same
  duplicate-host defect as `new` (1228 had not checked it).
- `close`, `open` and the other status verbs: local tracker only, no forge path
  (`cmd/harnez/issues.go:450`, `:486`).
- `edit`, `reopen`: no such verb.
- `fj` on this machine is a wrapper that already passes `-H codeberg.org`; `fj issue` has
  `create`, `comment`, `edit <n> title|body` (positional text only) and `close`, but no reopen.
- `tea` is not installed; `gh` 2.92.0 is. There is no fj-to-tea fallback.
- Comment body: the file must exist, but its content is never read; an empty body reaches the
  forge (`cmd/harnez/issues_forge.go:135`).
- Tests fake the CLIs with `stubIssueCLI` (`cmd/harnez/issues_forge_test.go:208`) and an
  executable fake `fj` (`cmd/harnez/issues_comment_test.go:47`).

Unverified after round 1: how a plain `fj` resolves the host without the flag, and the installed
fj version (the wrapper breaks `fj version`). Round 2 asks for read-only canaries.
