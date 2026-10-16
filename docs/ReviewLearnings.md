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
- **Verify before pushing**: never chain `test && push` in one command when the test output
  decides the push; read the result first.
