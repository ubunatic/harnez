# Review Learnings

Defects that reviewers caught repeatedly. Check these before asking for review.

- **Circular readiness**: when a client waits for "ready" before it sends anything, readiness must not
  depend on work that client's message triggers. Test the cold start with no clients. Issue 714/873.
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
- **Git integration tested beside the real flow**: green tests called the driver directly and
  asserted on the tree after hooks repaired it; real `init` installed nothing and the merge commit
  lacked rows. E2E through the real CLI in a scratch repo, assert on `git show HEAD:<file>` and a
  clean `git status`. Issue 847.
- **Edits to rendered files**: `.harnez/rules/*.md` are rendered from `config.yaml`, root docs from
  `docs/practices/` etc. via the installed binary. Edit the source, `make install`, then `harnez init -d .`
  and check that it leaves the edit in place. Issue 843 M3 edited only the rules output; the next init reverted it.
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
- **Swallowed server errors**: 830 M1/M2 answered `503 collector write failed` for every update while
  all tests passed; the writer got an empty DB path and the handler hid the cause. Log the error, and
  test through the real route into a store opened the production way. Issue 830 M2 rework.
- **Wire format by accident**: an untagged Go struct on an HTTP route makes `UsedFraction` the JSON
  key; adding tags later breaks stored JSONL (Go's case-insensitive match does not cover `_`). Tag
  wire types when the route is born. Issue 830 M3 pre-work.
- **Rewriting shared history**: other sessions push to the same main. Never amend, autosquash or rebase
  commits that exist on origin; fix with new commits. A rewrite made another session's pushed ticket look
  untracked and blocked all ticket commits (bug-hunt sprint 2026-10-22).
- **Verify before pushing**: never chain `test && push` in one command when the test output
  decides the push; read the result first.
- **Loosened assertions**: a "fixed the failing test" report can hide a weaker check (`HasPrefix` →
  `Contains` dropped the ordering the test name promises). Read every changed assertion line; find why
  the output changed (here a host `neus` call) instead. Issue 884, `4789aff4` → `67f8b396`.
- **Closed scope vs. goal**: compare what a milestone moved with its plan and the ticket's `/goal` before
  closing; 884 M9 moved 157 of ~5k planned lines and was first closed as delivered. Park the rest (888).
- **Daemon behaviour changes**: a test that runs a CLI which auto-starts the collector also exercises the
  collector. When the collector gains new work (879 M2: remote-load ssh polls), "the CLI never does X"
  assertions fail. Scope them to the CLI process (record the stub's parent PID), keep the env isolated
  (temp HOME/XDG dirs, own `local.yaml`, fake binaries first on PATH), and stop every spawned daemon in
  `t.Cleanup`.
- **Developer "open points" are goal checks**: a limitation the developer reports can be a goal violation.
  855 M2 listed "a later non-interactive run can re-enable a tracker disabled in the wizard" as an open
  point; the reviewer rated it a blocker (the goal says init keeps working from config). Check each
  reported open point against the ticket's `/goal` before closing.
