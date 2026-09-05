# 108 — Subagent dispatch needs a hard sequential-by-default rule + issue-number allocation race guard

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md), [docs/studies/2026-08-29-a-day-of-fresh-sprints.md](../docs/studies/2026-08-29-a-day-of-fresh-sprints.md), [issue 036](036-harnez-status-issues-tracker-linter.md) (tracker linter), issue 106, issue 107, commit `f6d8766`

## Incident (2026-08-30)

During a `/fresh-sprint` session, two subagents were dispatched close together
in time, each with an independent "investigation/ticket-filing only, no code
changes" task. Both subagents independently read `issues/README.md` to
determine "the next free issue number," and both landed on **106**. Each
filed its own ticket file — `issues/106-verify-offline-derivability-of-quota-state.md`
and `issues/106-indicate-data-staleness-via-dimming-marker-in-usage-ui.md` —
added its own row to `issues/README.md`, and committed independently. Result:
two files claiming issue 106, and two duplicate `| 106 | ... |` rows in the
README table.

The host orchestrator caught this after the fact by noticing both completion
reports referenced issue 106, and manually fixed it: renumbered the second
ticket to 107, cross-linked it, de-duplicated the README, and committed the
fix in `f6d8766`.

Notably, both colliding tasks were read-only/doc-only (ticket filing), not
concurrent code edits touching the same files. The earlier lesson in
`docs/studies/2026-08-29-a-day-of-fresh-sprints.md` ("What this says about
the harness itself") was specifically about concurrent *code* edits
converging on the same Go package (`internal/usage`) — "parallel is too
risky anyway" once tickets converge on shared files. This incident shows the
risk is broader than file overlap: any shared, sequentially-allocated
resource — here, "the next free issue number" derived by scanning
`issues/README.md` — can race even when the touched *files* don't literally
overlap, because the race is on a number picked by reading shared state, not
on the files each agent then writes.

This also directly extends the **Parallel Read, Sequential Write** invariant
in `docs/practices/AgenticLoop.md` (item 1): that invariant is framed around
"modify files, write code, or execute build mutations in a shared
workspace" as the write hazard. Allocating an issue number from a scan of
`issues/README.md` is a read followed by a write of a *derived* value (the
number), and the read-then-decide step is exactly where two concurrently
dispatched agents can both observe the same "free" state before either has
committed. The invariant's write-side is respected in isolation (both agents
wrote to *different* files, in the trivial file-path sense) but the shared
count they both derived from was stale by the time either wrote.

## Investigation: does `harnez status`'s tracker linter already catch this?

Checked `internal/issues/issues.go` (`LintFS`, called from `internal/claude/status.go`).

- **It already catches the actual symptom that occurred here.** The linter
  has a `DiagDuplicateNumber` diagnostic kind (line ~53) that counts
  occurrences of each issue number across `issues/README.md`'s table rows
  (`tableNumCount[row.Number]++`, checked at line ~345) and emits a
  diagnostic when `count > 1`. Since both subagents added a `| 106 | ... |`
  row, `harnez status` run after both commits landed *would* have reported
  this as `duplicate issue number 106 appears 2 times in table` — the
  collision was catchable by existing tooling, and was only missed here
  because `harnez status` wasn't run between the two commits.
- **But there is a real, confirmed gap underneath it.** `LintFS` also builds
  `numToFileMap := make(map[string][]IssueFile)` (line 237) while walking
  `issues/*.md`, populating it per file (line 274) — but this map is never
  read again anywhere in the function. It is dead for diagnostic purposes.
  So a duplicate *file-level* issue number (two files both prefixed `108-`,
  say) is only caught today as a side effect of both files' README rows also
  colliding — if only one row had been added (e.g. one agent forgot to
  update the README, or updated it and the other agent's commit raced and
  overwrote just that line), the file-level duplicate would go completely
  undetected by the current linter.

## Proposed fix (two parts)

### a. Behavioral/process rule (already in force)

Subagent dispatch defaults to **strictly sequential** — one subagent in
flight at a time — for **all** task types (code edits, investigation,
ticket filing), not just code edits converging on a shared package. Parallel
dispatch happens only on the user's **explicit** request for a specific
task, and even then the orchestrator must verify concurrency actually
happened and check for exactly this class of shared-derived-state race
afterward (duplicate ticket numbers, duplicate README rows, or any other
"scan shared state, derive next value, write" pattern).

This rule has already been saved to the persistent memory system as
`feedback_no_parallel_agents.md` as of this ticket being filed. This ticket
exists to harden it further (make it discoverable in the tracker, and pair
it with the mechanical guard below) — not to introduce it from scratch.

### b. Mechanical guard (proposed, not yet implemented)

Two independent options worth considering for whoever picks this up:

1. **Close the linter gap.** Extend `LintFS` in `internal/issues/issues.go`
   to actually use `numToFileMap` and emit a new diagnostic (or reuse
   `DiagDuplicateNumber`) when a single issue number is claimed by more than
   one file on disk, independent of what the README table says. This closes
   the confirmed dead-code gap above and makes duplicate detection resilient
   to partial/racing README updates, not just the lucky case where both
   racing agents also both remembered to update the README.
2. **Make allocation atomic by construction.** Consider a small
   `harnez issues new` (or similar) helper that claims the next free number
   transactionally (e.g. lock-file or git-index-based compare-and-swap on
   `issues/README.md`, or a monotonic counter file) rather than relying on
   every dispatch — human or agent — to freshly re-scan `issues/README.md`
   and hope nothing else is mid-flight. This is the stronger fix (removes
   the race instead of just detecting it after the fact) but is a real
   design task of its own — flagging it here as an option, not specifying
   an implementation.

Don't over-design either option in this ticket; pick one or both when
actually picking this up.

## Recurrence (2026-09-03)

`harnez status` flagged this exact symptom again, live: `issues/README.md` currently carries
two distinct files each claiming issue 179 (`179-harnez-rate-ok-heartbeat.md`,
`179-instruct-agents-on-harnez-find-in-agents-md.md`, renumbered to
[242](242-instruct-agents-on-harnez-find-in-agents-md.md) in issue 240) and issue 180
(`180-release-webextension-manifest-and-json-version-sync.md`, renumbered to
[243](243-release-webextension-manifest-and-json-version-sync.md) in issue 240,
`180-watch-and-review-compact-commands.md`) — both pairs pre-date this session's own ticket
work (202–208, which used `harnez find issues next --reserve` throughout and did not
collide). Left as-is rather than silently renumbered inline, since fixing it means picking a
canonical number for each pair and updating every cross-reference to the bumped one — that's
real work belonging to whoever picks up the mechanical guard in this ticket, not a drive-by
edit. Confirms the race is still live under real usage, not just a one-off from 2026-08-30.

## Acceptance Criteria

1. Document (this ticket) whether `harnez status` already catches duplicate
   issue numbers — confirmed above: yes, at the README-table-row level via
   `DiagDuplicateNumber`; no, at the file level, since `numToFileMap` is
   built but unused. If picked up for implementation, add the file-level
   check.
2. The sequential-dispatch behavioral rule stands as the primary mitigation
   (already in force via `feedback_no_parallel_agents.md`), with this ticket
   as its paired process-level documentation and the mechanical-guard
   options as the follow-up engineering work.

---

## Implementation Plan

### New finding: `Reserve` does **not** actually close the race

Since this ticket was filed, `harnez find issues next --reserve` shipped
(`internal/issues/issues.go:550`, `Reserve`). It looks like proposed-fix option 2, but
re-reading it shows it is not:

```go
baseName = fmt.Sprintf("%s-%s.md", nextNum, slug)   // number + *title slug*
...
f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
if os.IsExist(err) { continue }   // retry with the next number
```

The `O_EXCL` compare-and-swap keys on the **full filename**, i.e. number *plus slug*. Two
agents reserving number 106 with *different* titles produce two different filenames, both
`O_EXCL`-create successfully, and both "own" 106 — exactly the 2026-08-30 incident, and
exactly the 179/180 pairs the 2026-09-03 recurrence found. `Reserve` only defends against
two agents choosing the same number *and* the same title.

So option 2 is ~80% built and has one real bug; that is now the highest-value work here.

### Steps

1. **`internal/issues/issues.go` — make `Reserve`'s claim number-exclusive.**
   Add a sentinel keyed on the number alone, claimed before the titled file is written:
   ```go
   claimPath := filepath.Join(issuesDir, "."+nextNum+".claim")
   c, err := os.OpenFile(claimPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
   if os.IsExist(err) { continue }  // someone else holds this number, try the next
   ```
   Then write the real `NNN-slug.md`, then `os.Remove(claimPath)`. Also guard against the
   pre-existing-file case the current code misses: before claiming, reject `nextNum` if any
   `NNN-*.md` already exists (cheap, since `Scan` already walked the dir — check
   `numToFileMap`-style, not a second `filepath.Glob`).
   - **Stale-claim handling**: a crashed reserve leaves a `.claim` file that blocks a number
     forever. Treat a claim file older than ~60s as abandoned and steal it (`os.Stat` mtime
     check before the `continue`). Keep this simple — no PID tracking, no lockfile library.
   - Dotfiles are ignored by `ScanFS`'s `NNN-*.md` pattern, so claims never appear as
     tickets; confirm that when implementing and add a test for it.

2. **`internal/issues/issues.go` `LintFS` — use the dead `numToFileMap` (line 393-396).**
   After the existing `tableNumCount` duplicate loop (~line 464), add:
   ```go
   for num, files := range numToFileMap {
       if len(files) > 1 {
           // paths sorted for deterministic output
           diags = append(diags, Diagnostic{Kind: DiagDuplicateNumber, IssueNum: num,
               Message: fmt.Sprintf("issue number %s claimed by %d files: %s", num, len(files), strings.Join(paths, ", "))})
       }
   }
   ```
   Reuse `DiagDuplicateNumber` rather than adding a kind — same problem, different
   detection surface, and any consumer switching on the kind keeps working. **But** the
   existing table-level loop and this one will now both fire for the common case (both
   files also have README rows), producing two diagnostics for one problem. Either
   de-duplicate by skipping the table-level diagnostic when the file-level one already
   fired for that number, or (simpler, preferred) merge the messages: emit one diagnostic
   per number reporting both counts. Pick the merge — one problem, one line.

3. **Tests — `internal/issues/issues_test.go`.**
   - `LintFS` over an `fstest.MapFS` with `106-a.md` and `106-b.md` and **only one** README
     row: assert exactly one `DiagDuplicateNumber` for 106. This is the case that is
     undetectable today and is the whole point of step 2.
   - Both files *and* both rows present: assert exactly **one** diagnostic, not two.
   - `Reserve` concurrency test: `t.TempDir()`, N goroutines calling `Reserve` with
     *distinct* titles, `errgroup`/`WaitGroup`; assert the returned numbers are all
     distinct and that the on-disk `NNN-` prefixes are all distinct. This test fails
     against today's implementation and passes after step 1 — write it first.
   - `Reserve` with a stale `.claim` file present: assert the number is reclaimed.
   - `Reserve` leaves no `.claim` files behind on success.

4. **Clean up the live 179/180 duplicates** (the 2026-09-03 recurrence). This is
   bookkeeping, not code, and should be a **separate commit** from steps 1-3:
   for each pair, keep the earlier-committed file's number (`git log --diff-filter=A` on
   each file to determine which came first), renumber the other to the next free number
   via `harnez find issues next --reserve`, `git mv`, update its `# NNN —` heading, then
   `grep -rn "\b179\b"` / `\b180\b` across `issues/` and `docs/` to fix cross-references
   ("[[179-...]]" wiki-links included), then `harnez index` and confirm `harnez status`
   reports zero tracker diagnostics.

5. **`docs/practices/AgenticLoop.md`** — extend Invariant 1 (Parallel Read, Sequential
   Write) with one sentence naming the broader hazard this incident proved: a
   *read-then-derive-then-write* on shared state (the next issue number) races even when
   the written files don't overlap. Point at `harnez find issues next --reserve` as the
   mechanical answer. Two or three lines; do not restructure the doc.

### Design decisions / tradeoffs

- **Fix `Reserve`, don't replace it with a counter file.** A monotonic counter file would
  also work and is simpler to reason about, but it desynchronizes from the filesystem the
  moment a ticket is deleted or renumbered by hand (which step 4 is about to do), and the
  directory *is* already the source of truth. The claim-file fix keeps one source of truth.
- **Keep the linter even though `Reserve` will be race-free.** Numbers also get allocated
  by hand and by agents that don't call `Reserve`; detection is the backstop, not the fix.
- **Don't build enforcement of the sequential-dispatch rule.** It's a host-orchestrator
  behavior, already in memory and in this ticket; `harnez` has no lever over subagent
  dispatch. Part (a) is documentation-complete.

### Risks / open questions

- The concurrency test is the only thing proving step 1 works; make it deterministic
  (fixed goroutine count, no `time.Sleep`-based synchronization) or it becomes a flaky
  test that gets skipped.
- Renumbering in step 4 breaks any external link to `issues/179-...`. Solo repo, no
  external consumers — acceptable; mention the old number in the renumbered ticket body
  so a future grep finds it.
- Stale-claim stealing has a theoretical window where two agents both steal the same
  60s-old claim. Acceptable: the pre-claim "does `NNN-*.md` already exist" check catches
  the practical version, and the alternative (real advisory locking) is over-built for a
  solo repo's ticket numbering.

### Scope

**Small-to-medium** — steps 1-3 are ~60 lines plus tests in one file; step 4 is a
mechanical but careful cross-reference sweep best done as its own commit; step 5 is three
lines of docs.
