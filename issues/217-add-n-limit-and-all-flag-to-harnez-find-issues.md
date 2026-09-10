# 217 — Add -n Limit (Default: 10) and --all Flag to harnez find issues

**Status**: Closed — resolved in 9e31efd: find issues now defaults to newest 10, supports --limit/-n and --all, with ranked-search and bare-list tests
**Priority**: P1 (High) — bumped from P2 2026-09-07 after a second real occurrence (see Recurrence section); flagged as a recurring papercut across "any find/list command"
**Severity**: Minor
**Category**: Feature / CLI
**Related**: [158-find-entity-query-command.md](158-find-entity-query-command.md), [194-reserve-next-issue-number.md](194-reserve-next-issue-number.md), [docs/CLIDesign.md](../docs/CLIDesign.md)

---

## 1. Problem & Motivation

`harnez find issues` searches and lists issues (e.g. `harnez find issues status:open`).
Currently:
1. Running `harnez find issues status:open` prints every matching issue across the repository (often 50–100+ tickets), flooding the terminal.
2. In interactive agent or human workflows, the most common inquiry is inspecting the **most recent** issues or a limited batch of top results. Users currently have to pipe to `tail -n 10` or `head -n 10`.
3. Running `harnez find issues` without a query currently errors with `find: query must not be empty`, preventing a quick view of recently reported issues.

---

## 2. Technical Specification

### 2.1 CLI Flags on `harnez find issues`
Add flags to `cmd/harnez/find.go`:
- `-n, --limit <int>` (default: `10`):
  - Limits the output to at most *N* entries.
  - For empty or broad status queries (e.g. `harnez find issues` or `harnez find issues status:open`), returns the last *N* issues (highest ticket numbers / newest).
  - When *N* is specified explicitly (e.g. `-n 5` or `-n 25`), bounds results to *N*.
  - `-n 0` or negative value is rejected with a clear usage error.
- `--all, -a` (bool, default: `false`):
  - Bypasses the default `-n 10` limit, returning all matching results.
  - When `--all` is set, `-n` limit is ignored or unbounded.

### 2.2 Default Query Behavior
- Allow running `harnez find issues` without arguments (or with only flags):
  - Defaults to listing the last 10 issues (equivalent to `harnez find issues -n 10`).
  - When combined with status filters like `harnez find issues status:open`, limits output to the last 10 open issues by default.

### 2.3 Slice / Ordering Contract
- `harnez find issues` sorts numerically by ticket number when ranks match.
- For a query that lists issues (like `status:open` or bare query), taking the last *N* issues should select the most recent / highest numbered tickets (e.g., ticket 208, 209, 210... 217), keeping them cleanly accessible.
- For ranked text search (e.g. `harnez find issues "vram"`), limit selects the top *N* best matches according to the ranking contract, unless `--all` is specified.

---

## 3. Implementation & Verification Plan

- [ ] Update `cmd/harnez/find.go` flags (`-n`/`--limit`, `--all`).
- [ ] Update `cmd/harnez/find.go` argument parsing to allow empty query when flags like `-n` or `--all` or no query are provided.
- [ ] Implement slice truncation logic in `cmd/harnez/find.go` / `internal/find`.
- [ ] Add unit and CLI integration tests in `cmd/harnez/find_test.go` and `internal/find/`.
- [ ] Run `make check` and `make install`.
- [ ] Update documentation / help text in `cmd/harnez/find.go`.


---

## Implementation Plan

Code reviewed: `cmd/harnez/find.go` `runFind` (line ~250 onward) joins
`args[1:]` into a query, rejects empty with `find: query must not be empty`, then
prints every `find.Search` result unbounded. `find.Search`
(`internal/find/search.go`) sorts by worstClass, sumClasses, numeric ticket
number ascending, then path — so for a filter-only query (`status:open`) every
result shares class 0 and the order is purely ticket number ascending. That makes
"the last N" a plain tail of the sorted slice, no new sort needed.

### Design decisions

- **Truncate in `cmd/harnez/find.go`, not in `internal/find`.** `Search` is the
  ranking contract from issue 158; keeping it total and letting the CLI decide
  presentation preserves that contract and keeps the change to one file plus
  tests. Add a tiny exported helper only if a second caller appears.
- **Two different truncations, selected by whether the query has text groups**
  (`len(q.Groups) == 0`):
  - *filter-only or bare* → take the **tail** N (highest ticket numbers), keeping
    ascending order in the output so the newest ticket is the last line printed.
  - *ranked text search* → take the **head** N (best matches), since head is
    already "top N" under the existing sort.
  This matches §2.3 exactly and is the only place the two behaviours differ.
- **`--all` wins over `-n`** when both are given, rather than erroring —
  `--all` is an explicit "no limit" and treating the combination as a usage
  error buys nothing.
- **Bare `harnez find issues` becomes legal** and means "last 10". Keep the
  `query must not be empty` error only for the case where a query was *given*
  and parsed to nothing (e.g. whitespace-only quoted arg) — that is still a
  real user mistake. If that distinction proves fiddly, allow both; it is not
  load-bearing.

### Steps

1. `cmd/harnez/find.go` `newFindCmd`: add
   `cmd.Flags().IntVarP(&limit, "limit", "n", 10, "...")` and
   `cmd.Flags().BoolVarP(&all, "all", "a", false, "...")`. Thread both into
   `runFind` — its signature is already a long positional list; consider
   collapsing the flag args into a `findOptions` struct in the same commit,
   following the existing `findHistoryOptions` precedent in this file.
2. `runFind`: after the `next`/`history` subcommand branches (which must ignore
   `-n`/`--all` entirely), replace the empty-query error with the bare-query
   path; validate `limit` (`<= 0` → `fmt.Errorf("find: --limit must be >= 1")`)
   unless `--all` is set.
3. Empty-query path: skip `find.ParseQuery` and build a match-everything query,
   or short-circuit to "all scanned files as Results". Prefer the latter — do
   not add an empty-query special case inside `ParseQuery`, whose strict
   grammar-error behaviour (issue 158) is deliberate.
4. Apply the head/tail truncation described above, then print as today.
5. Update the command's `Long` help: document both flags, the default of 10, the
   head-vs-tail rule, and that `next`/`history` are unaffected.
6. Tests:
   - `internal/find/search_test.go` — unchanged (contract untouched); add a case
     only if a helper lands there.
   - `cmd/harnez/find_test.go` — default caps at 10; `-n 3` on a filter query
     returns the three highest-numbered tickets; `-n 3` on a text query returns
     the three best-ranked; `--all` returns everything; `-n 0` and `-n -1` error;
     bare `harnez find issues` returns the last 10; `find issues next` and
     `find issues history` ignore the flags.
7. `make check` (and `make install` per repo convention, at commit time).

### Risks / open questions

- **Silent truncation breaks existing scripted callers**, including harnez's own
  instruction blocks that tell agents to run `harnez find -d <repo> issues status:open`
  to list open issues — those will now see 10. Mitigation: print a one-line
  stderr note when results were truncated (e.g.
  `# 63 matches, showing 10 (use --all)`), keeping stdout's TSV contract
  byte-clean. Recommended; confirm it does not upset any parser.
- `-a` as the short flag for `--all` does not currently collide with anything on
  this command (`-d`, `-n` are taken), but check the global flag set.
- The instruction text in `config.yaml`'s "Issue Tracker Discovery" sections and
  `docs/templates/AGENTS.md` should mention `--all` once this lands, so agents
  know how to get the full list.

### Scope

Small (one CLI file, ~40 lines plus tests).

## Recurrence (2026-09-07)

Hit again live in `voxi` (a sibling harnez-managed project), doing ordinary
ticket-tracker work:

```
$ harnez find issues "is:open" -n 5
Error: unknown shorthand flag: 'n' in -n
```

The user's framing this time: "`-n` seems like a must for any find/list
commands" — i.e. this is not perceived as a `find issues`-only gap but a
general CLI ergonomics expectation (`head -n`, `git log -n`), and it should
be considered for every harnez subcommand that renders a list of results,
not only `find issues`. Workaround given was piping through `head -N`
manually, which works but doesn't match the user's own mental model of how
this tool should behave.

A pass over `cmd/harnez/` (2026-09-07) turned up other genuinely list-shaped,
potentially-unbounded outputs that would benefit from the same `-n`/`--limit`
convention once this ticket's design lands, beyond the `find issues` case
already scoped above:

- `harnez find issues history` (`cmd/harnez/find.go`) — renders every
  recorded issue-status snapshot, oldest first, unbounded.
- `harnez feedback list` (`cmd/harnez/feedback.go`) — lists every unreviewed
  feedback entry for a project.
- `harnez usage history timeline` (`cmd/harnez/main.go`) — renders the merged
  usage-history timeline across all recorded machine logs, unbounded.
- `harnez stats` (`cmd/harnez/stats.go`) — its `ByTool`/`ByAgent`/`ByProject`
  grouped breakdowns grow with the corpus and have no cap today.

Not proposing a full spec for all four here — this ticket's own §2/§3 design
(head-vs-tail truncation, `--all` escape hatch, truncation notice on stderr)
should stay scoped to `find issues` as originally planned and land first.
But whoever picks this up should treat "give `find issues` its `-n`" as the
reference implementation for a shared convention, and open follow-up work
(or fold it in here if trivial) to apply the same `-n`/`--limit int` flag,
consistently, to the other list-shaped commands above rather than
special-casing just the one command that happened to get hit twice.
