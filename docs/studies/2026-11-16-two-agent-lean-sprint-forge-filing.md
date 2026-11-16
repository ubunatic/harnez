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

## Owner decisions (2026-11-16)

Recorded in ticket 1202: forge issue writes go through harnez's own API client with a
write-capable stored token, fj stays as fallback for create and comment; forge writes only in a
host session, never in a subagent; the live check runs on `ubunatic/harnez` and `harnez/taia`.

## Developer M1 (1228, host-qualified fj references) and M2 (1212, empty body and token diagnosis)

Developer `forge-dev`, codex:terra:low. Plan turn 32s, 26.3k new tokens; the plan found that
`comment` rejects `-F -`, so stdin cases were dropped. Implementation turn 2m55s, 14.1k new tokens.

- Canary (read-only): `fj issue search --repo codeberg.org/ubunatic/harnez` exited 0 with both
  `~/.cargo/bin/fj` and the `~/.local/bin/fj` wrapper.
- M1 `00c70238`: one builder `forgetracker.FJIssueCommand` for create and comment; no host flag;
  `fj issue create --repo <host>/<owner>/<repo> ...` and `fj issue comment <host>/<owner>/<repo>#N ...`.
  A failed fj call prints the command to run by hand.
- M2 `135fb06a`: `issues comment` reads the body file and refuses an empty or whitespace-only body
  before any forge call; "token was already used" and the `keys.rs` panic give a one-line login
  hint; help text, help golden file and `docs/commands/issue.md` updated.
- Developer's `make test-q1` exited 0 with no `--- FAIL`; it quoted a `make` line, not a test
  summary line. Host rerun of the touched packages: `ok internal/forgetracker`, `ok cmd/harnez`
  (63s).
- Host check with the installed binary: `harnez issues comment -d . 41 -F <file> --forge --dry-run`
  prints `dry run: fj issue comment codeberg.org/ubunatic/harnez#41 --body-file "<file>"`; a
  whitespace-only file gives `Error: issues comment: body file is empty or whitespace-only`.

Host review findings (pre-work for the next milestone):

- No test asserts the "run manually" line for a failed `issues new` or `issues comment`.
- The two login hints differ (`fj auth login` and `fj -H <host> auth login`); the second fails with
  a wrapper that already passes `-H`, the first has no host for a plain fj.

## Architect

First dispatch on `--model smart` (claude:opus:low) failed at 08:26: Claude 5-hour limit. Retried
at 08:45.

The retry returned the design that ticket 1202 now holds: harnez's own Forgejo writer in
`internal/forge`, a planner in `internal/forgetracker` that picks API or fj, a host-only check in
the command layer, and milestones M3 to M8. It stated the limit of the host-only check itself:
"a guard against accidents, not a security boundary" (a native host subagent, a worker that unsets
the variables, or a worker calling curl or fj directly are not stopped).

## Developer M3 (write-token spec and CLI) and M4 (Forgejo issue writer)

Developer `forge-dev` (codex:terra:low), one write turn, three commits: `a262a1dd` (pre-work from
the M1/M2 review), `97c0ec9c` (M3), `b77e1634` (M4). Its `make test-q1` exited 0. The host reran
the four touched packages (`ok`, `cmd/harnez` about 63s). `forge-dev` was deleted after M4.

Reviewer `forge-rev2` (one-shot, read-only) on `a8cd7d84..b77e1634` produced seven pre-work items
for M5: transport errors lost their cause, the forge message was shown only for 412,
`Writer.Comment` returned no URL, `LabelIDs` read one page, no test for a 200 reply or the
returned fields, a nil client meant no timeout, and the token CLI isolation was tested through a
helper and not the command.

## Developer M5 (host-only check; `new` and `comment` over the API)

Fresh developer `forge-dev2` (`--model lean`, codex:terra:low).

| Turn | Time | Result |
|---|---|---|
| Plan (read-only) | 44s | Plan matched the ticket; context 46.1k of the 100k compaction limit |
| Write 1 | 1m55s | Stopped with three uncommitted files, no commit, no blocker named |
| Write 2 | 9m29s | `b2f68047` (pre-work), `624510c8` (M5); `make test-q1` exit 0 |

- Write 2 re-read 6.3M cached tokens for 71.1k new ones (88x, harnez's own check line); the
  session compacted once during the turn (context 27.9k afterwards).
- The first write turn ended early without a stated reason. The resume prompt that worked said:
  both commits in this turn, stop only for a real blocker and name it.
- Incident: a new worker-denial test did not replace the CLI runner and ran the real
  `fj issue comment codeberg.org/o/r#7`. fj answered "not found"; the repository does not exist
  (anonymous API GET: 404), so nothing was written. The handoff had said "never run fj"; the rule
  did not stop a test from doing it. The fix is structural: a `TestMain` guard (M6 pre-work).
- Host check that would have caught it: rerunning the packages with fake `fj`, `tea` and `gh`
  first on `PATH` that log every call. The log stayed empty for the committed tests.

Reviewer `forge-rev3` (one-shot, 1m27s, 67.5k new tokens) on `810c409d..624510c8` found what the
host's diff review had not: no package-wide guard against unstubbed CLI calls; `new --remote`
reached gh without the host-only check; `comment` on a GitHub forge did its issue GET before the
Forgejo-only refusal; dry runs always printed the fj command, also with a stored write token;
several weak assertions (`HTTP ` prefix only, no "fj never called" on the API path). All became
pre-work for M6 in the ticket.

## Developer M6 (`edit`) and M7 (`close`, `reopen`)

Same developer `forge-dev2`, resumed.

| Turn | Time | Result |
|---|---|---|
| Write 1 | 1m26s | Stopped: its own paging test from M5 failed after the ticket changed the paging rule |
| Write 2 | 9m23s | `00ef6564` (pre-work), `58c24095` (M6), `af87b676` (M7); `make test-q1` exit 2, fixed without a rerun |

- The first stop came from the host prompt: "stop for a failing test you did not touch" was read
  literally for a test the developer wrote one turn earlier. The wording that worked: when the
  ticket changes a behaviour, update the tests that assert the old one and name them; only a
  failure in an unrelated test is a blocker.
- Host full run (`make test-q1`, fake `fj`, `tea`, `gh` first on `PATH`): exit 0, 78 `ok` lines.
- Blocker found by the host with the installed binary and, independently, by reviewer
  `forge-rev4` (one-shot, 1m26s, 63.2k new tokens): the forge dispatch for `edit`, `close`,
  `reopen` and `open` was placed in the `Args` validator of the cobra command, so the local verb
  ran afterwards. `close --forge --dry-run 41` printed the dry-run line and then
  `041: already Closed (no change)`. All tests were green: the success tests called the helper
  functions directly and never went through the command.
- `close --forge` without a number panicked (`slice bounds out of range [2:1]`).
- Context at the end of write 2: 90.4k of the 100k compaction limit, 4.7M cached tokens re-read
  for 52.5k new ones (89x). `forge-dev2` was rated 3 of 5 and deleted; the M8 pre-work and docs
  went to a fresh developer `forge-dev3` on `--model std` (codex:terra:med).

Lessons so far:

- Green tests plus a clean diff review did not show the `Args`/`RunE` double run; one dry run of
  the installed binary did. Run every new verb through the installed binary before the review.
- A handoff rule ("never run fj") does not bind a test the worker writes. A guard in `TestMain`
  and fake CLIs first on `PATH` do.
- Stop conditions in a prompt are followed literally. Name what is not a blocker.
