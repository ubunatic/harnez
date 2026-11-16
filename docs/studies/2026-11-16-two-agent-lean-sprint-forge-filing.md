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

## Searcher round 2: fj host handling, credentials, harnez API client

`forge-scout` resumed after compaction (67.2k to 20.1k context usage), 1m18s, 51.8k new tokens.

- Forge path selection: tracker values are `harnez` and `none` (`spec/schemas/repo.schema.json:12`);
  the forge path needs no local `issues/` directory and a recognised origin
  (`cmd/harnez/issues_forge.go:61`); `comment --forge` overrides the directory check.
- fj is v0.7.0. Read-only canary on origin PR 78: the plain binary fails on `owner/repo#N`
  ("cannot find repo, no host specified") and works with `-H`; the host-qualified reference
  `codeberg.org/owner/repo#N` works with both the plain binary and the wrapper. Verified for
  `issue view` only; `issue create --repo <host/owner/repo>` is unverified.
- No environment variable or config file for the fj host was found in the help texts.
- fj credential storage and the `keys.rs:154` logic stay unverified (source not on disk).
- Harnez has its own forge API client with token lookup: stored Forgejo token, then
  `CODEBERG_TOKEN`/`FORGEJO_TOKEN`, then `gh auth token`; anonymous reads work
  (`internal/forge/api.go:43`, `spec/pr.yaml:20`). It only reads issues and pulls today; the only
  write requests are for Forgejo user tokens (`internal/forgetoken/api.go:95`).
- Codeberg swagger: issue PATCH takes `title`, `body`, `state` and `updated_at`; comment POST takes
  `body`. Accepted `state` values and any conditional meaning of `updated_at` are unverified.
