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
- **Only cite numbers read in the same turn**: a commit hash, count or measurement in a report comes from a
  tool output of that turn, never from memory. 852 (2026-10-24): the host reported "commit 2f…" before the
  lookup; the real hash was 98badd77.
- **Schemas before consumers**: 945 (formats) and 941 (loader + enforcement on every entry point) took 3 and
  6 review rounds because schemas were designed before any code used them and one ticket bundled four
  concerns. Cut epics into thin vertical slices with one visible result each; mark early schemas
  provisional. Epic 942 ## Re-cut.
- **A stopped run is not a green run**: 941 M4 committed after `make test-q1` stopped at gofmt, so the
  suite never ran; the host's full run found 3 failures. Grep the log for the test summary, not only for
  `--- FAIL`. In 941 M5 a "fixed" pin test was flipped from a negative to a positive case; M6 restored it.
- **Live-check through the running service**: 987 M4b's new telemetry record kind passed every test but
  showed `null` live, because the collector was the build from before `make install`. After installing a
  change that adds a record kind, run `harnez server restart` before the live check (989 tracks a fix).
- **Inherited identity**: a long-lived interactive host can inherit another worker's role/model/session.
  Verify exact thread metadata and launcher provenance; exercise actual reviewer dispatch and the
  child's refusal through the installed binary. A host health check alone misses dispatch failures.
  Issue 953 added `cmd/harnez/agent_caller_test.go`; 965 tracks stale issue-closure attribution.
- **Search evidence attribution**: an answer about live index freshness needs actual index diagnostics;
  a citation to a spec file does not support it. Keep exact questions, tool arguments, source generation,
  backend and raw answers; separate abstentions, partial retrieval and unsupported citations. Label
  queries run after an edit as retrospective comparisons. Session evidence is in Cati's
  `docs/studies/2026-10-26-search-mcp-neus-comparison.md` (2026-10-26 Harnez backfill).
- **Shared Git index**: commits in concurrent sessions can capture each other's staged paths.
  During the 953 evergreen follow-up, the 965 tracker commit was rejected because the index
  referenced another session's untracked 964; that session then committed both staged tickets.
  Check `git show --stat` before retrying and preserve the other session's history. Proposed harness
  improvement: serialize staging and commit as one cross-session operation; no AGENTS rule or lock
  implementation was added in this session.
- **Host orchestrator delegation boundary**: Under `/goal` or after context compaction, host agents default
  to inlined implementation unless the epic command documentation explicitly states that the host
  orchestrates sprints and runs post-handoff verification tests, but does not *do* the sprint implementation
  itself. Epic 999 refined `docs/commands/epic.md` to clarify that the Developer model defined in the
  selected sprint skill implements the changes, while the host only reviews diffs and runs verification checks.
- **Reviewer selection for complex diffs**: Use `argon:low` (`agy:argon:low`) for one-shot, no-digging full-diff
  reviews when complex work needs to be assessed. It provides high-signal detection of model trait fallbacks,
  schema boundary constraints, and test coverage gaps without conversational fatigue.
- **Mutation-check a new side effect's wiring** (1074): The Jules sync had passing direct tests while
  disabling its call in `Run`, its fetch, or adding `+` to its refspec left every test green. A reviewer
  found this by mutating the code. Assert the call from the real entry point and pin exact push arguments.
- **Context timeouts do not bound a git child holding the pipe** (1074): killing `git` left a hook's child
  holding stderr, so `cmd.Output()` waited 20 s past a 1 s timeout. Set `cmd.WaitDelay` and test with a slow hook.
- **Test hook scripts for every push outcome, including exit codes** (1074): the first `pre-push` guard looked
  right by ref state, yet a topic push through a two-URL remote exited 1 although Codeberg accepted it, a no-op
  push failed, and `harnez release` preflight would have aborted. Cover no-op, dry-run and rejected pushes.
- **Developer "partly verified" means a review round** (1061): a cheap developer's self-report named its own
  acceptance gaps. An independent reviewer rated them, and mutations showed which tests were real. Budget a
  second round, and rate each gap against the ticket's `/goal` before dispatching fixes.
- **Peer relays are not owner approval** (1084 sync, 2026-11-01): a peer session relayed "as recommended" and
  later "decide" for a multi-repo commit, each with its own narrower reading. The owner's direct "go" decided;
  the second relay arrived after the work was done. 1085 tracks whether `/decide` changes this.
- **Multi-repo commits by mtime window** (1084): selecting paths by the sync's timestamp and committing by
  explicit path kept other sessions' work out of 47 repos. Repos can fail their own hook on old lint errors:
  unstage and report; never bypass it. After `merge=union` conflicts, run `harnez index` on the branch.
- **Forge fields lag or mislead** (PRs 66, 68): the API showed `changed_files: 0` on a 33-file PR, and
  `mergeable: false` right after a push. Compare the `.diff` and re-check before acting on them.
- **Go hides a missing working dir as a missing binary** (1117): with `exec.CommandContext` + `Setpgid` +
  `StdoutPipe` + `Start`, a removed `c.Dir` fails as `fork/exec <binary>: no such file or directory`; a
  plain `Run` says `chdir`. Reproduce with the exact exec shape before ruling a cause out. `startProcess`
  now checks the dir first (`internal/subagent/codex_dir_test.go`).
- **Forgejo `q` is not a substring search** (1116): `q=asskey` returned 37 issues, `q=passkey` 50. Forge
  tracker find (`internal/forgetracker`) filters client-side within the `open_pr_scan` page budget and
  reports partial results (`truncated` in JSON). Canary a search API before relying on its semantics.
- **Review the output contract, not only the logic** (1116, two haiku rounds): reviewers found that
  `--json` had no truncation signal and that the CLI wiring had no test. Ask for both explicitly.
- **A test that reads the live machine fails only on some hosts** (1175): the usage watch frame test
  passed no `Load`, so host load changed the Load box width and repacked panels. Inject every live
  input as a fixture; a test that fails on one machine and not in CI is an environment leak first.
- **Green tests hid a visible regression** (1175 M1): the geometry fix passed every test but clipped the
  live Load row with free columns left. Render the installed binary in a PTY at more than one width.
- **A one-shot cheap reviewer pays for itself** (1177, 1138, 2026-11-06): codex:terra:low reviews of
  24-43s found a weak test, golden-coverage gaps, an over-wide stable API and two untested promised
  behaviours that host review had passed. Run one per milestone diff.
- **Batch audits overclaim "done"** (Jules cleanup 1140-1170): 2 of 21 proposed closes held in the first
  wave; Jules matched titles, not the done-when. Check each close against the ticket's `/goal` and run it
  live where cheap; many "done" tickets were done under another ticket's commit (close with
  `--implemented-by`, 1174).
- **A routing check on raw args misses quoted and flag forms** (1198, argon review 2026-11-09): the forge
  path matched `args[1]` against local-only words, so `"closed bug"`, `--next` and the DB-only
  `history` took the wrong branch. Split args into words before matching, route flags too, and send
  subcommands that never read `issues/` past the forge switch. Tests: `TestFindForgeClosedWordAndActiveAlias`.
- **Gemini argon:low as one-shot reviewer** (2026-11-09): 6 min, 3 real medium findings in a diff a
  Haiku reviewer and the host had passed. Slower than codex low reviews; worth it for a whole-session sweep.
