# Review Learnings

Defects that reviewers caught repeatedly. Check these before asking for review.

- **Untrusted text to a terminal**: model and tool output may carry escapes. Keep SGR only, and test
  controls hidden behind grapheme joiners (`a‍\x1b[2J`). Issues 779, loom 322.
- **Error-text matching**: match only the provider's own stderr, at line start, with CLI-verified
  patterns in spec. A false positive that drops state is worse than a visible error. Issue 791.
- **Unbounded queues**: a producer posting per chunk to a UI loop (`Pane.Post`) can exhaust memory
  while the loop is paused; coalesce to one pending post. Issue 778.
- **Shared context**: never `SetContext` on a shared `*cobra.Command` while goroutines read it; pass
  `ctx` down. Issue 794.
- **"Done" before exit**: a worker that records completion before exiting makes `wait` return early;
  track the PID until the process is gone. Issue 795.
- **Unjoined goroutines in tests**: background writes into `t.TempDir` fail cleanup; expose a join
  (`WaitObservations`). Issue 792.
- **Timing budgets under `-race`**: scale with a `raceEnabled` build-tag const; prove concurrency
  with a barrier, not wall clock. Issues 793, 796.
- **Files read while written**: `os.WriteFile` truncates first; a polling reader sees an empty file.
  Write a temp file and rename (`FileSessionStore.Save`, CI run 39011231816).
- **Tests that write tracked files**: a fixture generator in a test rewrites `testdata/` (cati 101).
  Tests read fixtures only; regenerate behind `-update`. A read-only container mount catches it.
- **Package-level test hooks**: a test swapping a package var while an earlier test's goroutine
  still reads it is a data race (cati mediabrowse). Inject hooks per instance; join background
  work in `t.Cleanup`.
- **PTY output after exit**: `Wait` must drain the PTY to EOF, not return on child exit (loom 329).
- **Unverified worker claims**: reproduce a worker's stated blocker before accepting a change
  built on it (voxi go.mod revert e4fe669).
- **Readers that write**: a "read" command that runs schema DDL, migrations or history writes on
  every call contends with real writers and fails with `database is locked`. Readers open `mode=ro`;
  one process writes. Issues 828, 829.
- **Green suite, wrong numbers**: after 829 M2 all tests passed while the live header showed a
  15-minute-old time and Claude tokens of 4.2 billion (was 18 million). Run the installed binary and
  compare magnitudes against the previous output. Issue 829 M3.
- **Verify before pushing**: never chain `test && push` in one command when the test output
  decides the push; read the result first.
