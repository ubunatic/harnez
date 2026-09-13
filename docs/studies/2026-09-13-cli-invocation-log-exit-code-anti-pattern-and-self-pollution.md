<!-- harnez:topic: harnez log feature arc (cli_invocations, attribution, log verb), the os.Exit-in-RunE anti-pattern it exposed, and a telemetry self-pollution bug found while dogfooding it -->

# `harnez log`, the `os.Exit`-in-`RunE` Anti-Pattern, and a Telemetry Self-Pollution Bug

**Scope**: Building issues 326/327/328 (`cli_invocations` telemetry, human/agent
attribution, the `harnez log` verb), the `os.Exit` cleanup it motivated across six
existing command sites, and a real bug the new feature's own dogfooding surfaced —
`go test ./...` silently writing into the developer's actual `~/.harnez/tool_catalog.sqlite`.

**Accessed**: 2026-09-13

## Finding 1 — an owner idea, correctly narrowed by an advisor before any code

The owner's framing ("a `harnez log` like `git log`, track everything harnez has done,
including human-run calls") was broad enough to invite real scope creep: a naive reading
would rebuild `find issues history`, `stats`, and `dochistory` under one new name. Dispatching
an Opus advisor first — read-only, no code — to evaluate feasibility against the actual
code (not the idea in the abstract) paid off directly: it found that repo-content history is
already fully covered by git + existing commands, that CLI *invocation* history genuinely
doesn't exist anywhere, and that "track human calls too" was nearly free (`resolve.Session`'s
`ppid-` fallback and `detectAgent`'s `"unknown"` sentinel already carry the two signals
needed — they just weren't classified as such). It filed three right-sized tickets (326 write
path, 327 read command, 328 attribution) instead of one kitchen-sink ticket, each with an
explicit "what this must NOT do" section. This is the same shape of value issue
318's git-log-shorthand ticket had — advisory work that changes the scope of the
implementation, not just rubber-stamps it.

## Finding 2 — `/lean-sprint` on three sequentially-dependent tickets worked, with one real gap the dev subagent flagged honestly

A single dev subagent implemented 326 → 328 → 327 in order, on the actual branch (no
worktree, per this project's standing convention), self-verifying against every checkbox
in all three tickets' Verification sections before reporting back. The report was
detailed enough to review without re-deriving anything from the diff — file list, which
checklist items had test-level proof (naming the test), which had a live-terminal check,
and explicit deviations from spec with reasoning (the `Filter` type needed two
where-builders, not one; `--all` was extended to drop project scoping, not just the cap).

The one thing it flagged and did not fix: **retention-prune overhead was not benchmarked**
against the existing `rate_overhead_test.go`/`telemetry_bench_test.go` baselines, called out by
name in the ticket's own verification checklist. That is a case of "surface an unmet
checklist item explicitly" working exactly as intended — better than silently marking it
done, but the item genuinely still needs doing when it matters (this table's write load).

Inline review (not a second reviewer subagent — confidence was high, the report was
concrete and checkable) reproduced every claimed behavior live rather than trusting the
report: `find issues -3`/`issues list -3`/`--failed`/`--human` filters, exit-code recording
on a deliberately-failing subcommand, and a real human-vs-agent classification from an
actual pty. Nothing was wrong. This matched the lean-sprint's own "confidence-gated" design:
routine scope, clean tests, no cross-subsystem surprises → skip the separate reviewer,
verify inline instead.

## Finding 3 — building a "what did harnez run" log surfaced a real anti-pattern in six existing commands

Committing 326/327/328 immediately exposed that `index --check`, `issues --check` (×2),
`diff --exit-code` (×2), `exec`, and `distill`'s subprocess wrapper all called `os.Exit`
directly inside `RunE`. This was not a style-only issue: `os.Exit` terminates the process
immediately, so **nothing after it in the call stack runs — including the new
`executeAndRecord` write path itself**. Every one of those six commands was therefore
*invisible* to the very feature just shipped, not merely mis-recorded.

A websearch confirmed current Cobra guidance
([spf13/cobra#2124](https://github.com/spf13/cobra/issues/2124),
[#837](https://github.com/spf13/cobra/issues/837)) and the
[square/exit](https://github.com/square/exit) convention: pick the exit code once, at the
application boundary (`main()`, after `root.Execute()` returns), never inside command
logic. The concrete mechanism that made this clean rather than a large refactor was one
already documented on a cobra maintainer's own comment thread: **`SilenceErrors` and
`SilenceUsage` are read by Cobra at print time, not at command-construction time**, so
setting them from deep inside `RunE`, immediately before returning a sentinel error, scopes
the silencing to that one return path and leaves the same command's other, genuine error
returns printing exactly as before. `exitCodeError` (`cmd/harnez/exitcode.go`) plus
`silenceIfExitCode` is the resulting ~60-line shared primitive; six call sites became
one-line changes.

**A live smoke test caught something `go test` alone would not have**: after the first
pass, `distill -- false` printed a full `Usage:` block on stderr even though the exit code
was correct — `SilenceUsage` had never been set on that command (unlike `exec`, which
already had it), so returning an error through the normal `RunE` path re-exposed Cobra's
default usage-dump behavior that the old `os.Exit` call had always bypassed. Unit tests
never exercise `root.Execute()`'s actual print path, so this was only visible by running
the real built binary end-to-end per command. Fixed by having `silenceIfExitCode` set both
flags, not just `SilenceErrors`.

## Finding 4 — the new feature's own dogfooding found a pre-existing test-pollution bug

The user asked, essentially, "are we polluting our own stats by running our own test
suite?" — a good question to ask of any self-observability feature. The answer was yes:
`cmd/harnez/exec_test.go`'s `TestGearMulticallExecution` builds the real `harnez` binary
and runs it as a subprocess three times, and had set `HARNEZ_DB_PATH`/`HARNEZ_STATE_DIR`
env vars that **no production code has ever read** — `telemetry.DefaultDBPath()` and
`resolve.DefaultStateDir()` both resolve purely from `os.UserHomeDir()`, with zero override
hook. Confirmed empirically, not just by code inspection: running the test moved the real
`~/.harnez/tool_catalog.sqlite` row count by exactly 3, attributed to project `"harnez"`
(the subprocess's cwd is this package directory) — indistinguishable from genuine usage.
Before issue 326, this only polluted `tool_calls`; after it, every `go test ./...` run was
also writing real `cli_invocations` rows.

The fix used the one isolation mechanism that actually exists — `t.Setenv("HOME", ...)`,
matching the only other subprocess-boundary test in the repo
(`internal/claude/bash_shim_test.go`) — rather than building real production support for
path-override env vars just to satisfy one test. That would have been solving a narrower
problem with a bigger, permanent surface: a `HOME` override isolates *everything* rooted
under the user's home directory in one shot, including anything a future feature adds
there later without remembering to also wire a new override; a `HARNEZ_DB_PATH`-only fix
only ever covers what it explicitly names. Fixing the isolation also exposed a second,
independent bug: the test's own verification of the telemetry row was checking the (now
never-populated) fake path, got an empty database back, and its real assertions lived
inside an `if len(rows) > 0` guard — so they had never actually run. The test was green
without checking anything.

**Known unresolved fact, not yet acted on**: the real `~/.harnez/tool_catalog.sqlite` on
this machine already carries an unknown amount of historical pollution from `go test`
runs before this fix (some of the `harnez|exec|60` / `harnez|exec hook|45` counts observed
during this session are almost certainly test-run noise, not genuine usage). No retroactive
cleanup was performed — see issue (to be filed) below.

## Related tickets

- Issues 326, 327, 328 (closed, `ef5dd30`) — the `cli_invocations`/attribution/`log` arc.
- The `os.Exit`-in-`RunE` fix (`4ce6a91`) and the `TestGearMulticallExecution` isolation fix
  (`486211c`) were not separately ticketed — both were same-day, same-context follow-through
  on 326's own dogfooding, small enough to fix directly per this project's "small fixes: direct
  commit once tests pass" convention.
- New: historical telemetry pollution cleanup/quarantine for `~/.harnez/tool_catalog.sqlite`
  (see issues/README.md for the number filed alongside this study).
- New (optional, explicitly not recommended for now): a real `HARNEZ_DB_PATH`/
  `HARNEZ_STATE_DIR`-style production override, filed as a Draft idea rather than acted on —
  see that ticket for why the assessment leaned against building it just to fix the test.

## What I'd do differently

- When a subagent's own verification checklist names a specific unmet item (the retention
  prune overhead benchmark here), convert that into a tracked follow-up the moment the work
  is reviewed, rather than letting it live only in a chat transcript that gets compacted away.
  It surfaced again in this write-up only because this session happened to still be live.
- Live-smoke-test every command whose `RunE` path changes shape (not just the ones the
  ticket's verification checklist explicitly calls out), specifically by actually running the
  compiled binary and reading stdout/stderr — the `distill` usage-dump regression and the
  `TestGearMulticallExecution` pollution were both real bugs invisible to `go test ./...`
  passing cleanly, and both were only found by treating "tests pass" as necessary, not
  sufficient, per `docs/practices/AgenticLoop.md`'s own "Unit-Test-Only Confidence" anti-pattern.
- Before adding any new best-effort telemetry write path that fires on every invocation, grep
  for existing subprocess-spawning tests in the same command tree first — a new write path
  inherits every isolation gap those tests already had, silently making a latent bug louder.
