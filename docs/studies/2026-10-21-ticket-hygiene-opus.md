# Ticket Hygiene at ~900 Tickets — Researcher R2 (opus)

**Date**: 2026-10-21
**Ticket**: [842](../../issues/842-research-ticket-hygiene-at-900-tickets-archive-backlog-tiers-short-open-lists-duplicate-detection.md)
**Scope**: read-only research; three solutions within the file-based `issues/*.md` tracker. No code.

## 1. Observed Baseline (measured 2026-10-21)

- 820 files in `issues/` (+19 in `issues/archive/`, empty `issues/inbox/`). Most closed tickets
  were never moved: the archive convention in docs/IssueTracking.md §4 is effectively unused.
- `harnez find issues -a status:open -t` → 248 rows, **~41 KB (~10k tokens)** per listing.
  Without `-a`, listings default to the newest 10, so the cost lands on `-a` and on README reads.
- `issues/README.md` is **246 KB** (~60k tokens): every ticket ever, one row each. Any agent that
  opens it (it is the obvious "index") pays far more than the open list.
- Open set by priority: 9 P1, ~156 P2, ~72 P3, 1 "P4". Category: ~60 Agentic Ergonomics, 53
  Feature, 25 Bug. 7 Blocked, 4 In Progress, 18 Draft.
- Last-touch age of 235 Open/Blocked files: 80 untouched for >30 days. Creation (per git)
  2026-08: 16, 2026-09: 160, 2026-10: 59 — the backlog is young but grows ~60/month.
- Status-line drift: `Done`, `Resolved`, `In Review`, `Closed (…)`, and old `**Status:**`
  (colon inside bold) variants exist. Any tier scheme must first normalize these.
- Duplicate search today: `harnez find issues` already scans active **and** archived tickets in
  every status; the `/issue` skill runs one `find issues "<terms>"` before `issues new`. Gaps: the
  query is AND-only (a long title query often returns nothing), nothing forces the step, and there
  is no "reopen/promote instead of new" verb or prompt.
- Lifecycle values are Go constants (`internal/issues/issues.go:45`), not in `spec/`; a new
  tier value should go into `spec/` per docs/Spec.md.

Sample of old open tickets (005, 007, 053, 128, 141): all still plausible, none clearly obsolete,
100–220 lines each, mostly P2 ideas or audits with no owner or next step. They are "someday"
items, not active work — the core hygiene problem is lack of a backlog state, not dead tickets.

## 2. Shared Building Blocks (used by all three proposals)

1. **Compact listing format**: `find issues` TSV is `num<TAB>status<TAB>title<TAB>path`; dropping
   the path and long status suffixes cuts ~40 %. Cheap win independent of the chosen scheme.
2. **Similarity search for duplicates**: `harnez find issues similar "<title or text>"` — OR over
   stop-word-filtered tokens, ranked by token overlap (title weighted > body), top 5, all tiers,
   tier/status column shown. Reuses existing normalization and fuzzy matching.
3. **Dup gate in `issues new`**: before reserving, run `similar` on the title; print the top
   candidates to stderr with their tier. `--force` (or a `--not-dup-of 123,456` note) skips.
   The `/issue` skill and docs/IssueTracking.md say: on a real match, promote/reopen the old
   ticket and append the new context there instead of filing.
4. **Grooming criteria** for migration (applied once, then as a periodic `issues groom` report):
   - stay active: P0/P1, In Progress, Blocked with live reason, touched in last 14 days, or
     referenced by an active ticket/roadmap (234);
   - backlog: P3; Draft >14 days; P2 untouched >30 days; Research tickets whose report exists;
   - close: superseded/duplicate (found via `similar` pairs during migration), "partially
     addressed" tickets whose remainder fits in a new short ticket.
   Rough estimate: these rules give ~70–100 active; reaching ~20 needs a human ranking pass.

## 3. Proposal A — Backlog as a Status Value (in place, no file moves)

**Mechanism**: add lifecycle value `Backlog [— reason]` beside Open/Blocked/Draft. Files stay in
`issues/`. New verbs `harnez issues backlog <n…> [reason]` and `promote <n…>` (= `open` with
an audit note). Closed tickets may still optionally move to `issues/archive/` as today.
**Listing/search**: `is:open` keeps meaning Open + In Progress + Blocked → excludes Backlog.
`is:backlog` and `-a` with no status filter reach it explicitly. `issues/README.md` splits into
an active table (top) and a link to generated `issues/BACKLOG.md` / `issues/CLOSED.md`, so
the README shrinks to the active rows (~10 KB at 100 tickets).
**Duplicate flow**: gate (§2.3) shows Backlog/Closed matches; agent runs `issues promote 123`
(Backlog → Open) or `issues open 123 "reopened: <why>"` (Closed → Open) and appends context.
**Migration**: one batch commit per criterion: `issues backlog 053 128 141 … "groomed 2026-10"`
(batch verbs already exist). Status normalization first.
**Tooling/docs**: status enum (+spec), `CanonicalizeStatus`, `find` filter, `index` split,
lint parity, two verbs, IssueTracking.md(+lite) §3/§4, `/issue` skill. Other harnez projects get
the new status via `harnez init`; projects without Backlog tickets see no change.
**Pros**: smallest change; no path churn, every link and `[[slug]]` stays valid; reversible by
one status edit; git history of a ticket stays in one path; batch verbs make migration trivial.
**Cons**: `issues/` stays a 800+ file directory — agents using plain Glob/Grep/`ls` (not harnez)
still see everything; "Backlog" vs "Open P3" overlap needs a clear rule.
**Risks**: agents keep filing new P3 work as Open → backlog regrows; needs a lint warning
when active count exceeds a budget (e.g. 100) — a soft WIP limit configured in `spec/`.

## 4. Proposal B — Directory Tiers (`issues/`, `issues/backlog/`, `issues/archive/`)

**Mechanism**: location is the tier. `issues/*.md` = active only (budget ≤100, target ~20);
`issues/backlog/*.md` = open but parked; `issues/archive/*.md` = all Closed tickets (enforce
the existing §4 convention, which today is unused). Status line stays the lifecycle; lint checks
location/status consistency (Closed must be in archive, etc.). Verbs `issues backlog <n>` and
`promote <n>` do `git mv` and rewrite path links (reuse the `mv` link-rewrite code);
`close` moves to archive, `open` on an archived ticket moves it back.
**Listing/search**: default listings and Scan for listings read `issues/*.md` only. `find issues`
*search* keeps scanning all three dirs, showing a tier column; `--tier active|backlog|archive`
narrows. Three generated READMEs, one per dir; the top README lists active only.
**Duplicate flow**: gate (§2.3) searches all dirs; on a match, `issues promote 123` moves the
file back to `issues/` and opens it. The physical move makes the promotion visible in git.
**Migration**: (1) move ~560 Closed tickets to `archive/` (one mechanical commit); (2) groom
the ~247 open via §2.4 into `backlog/`. Link rewrite across docs/ and issues/ touches many files.
**Tooling/docs**: Scan dirs, index (3 tables), `findTicketFile`, mv link rewriter, lint rules,
`new` writes to `issues/`, IssueTracking.md §4 rewritten, `/issue` skill. Other projects using
`issues/archive/` already fit; `backlog/` is new.
**Pros**: strongest context guarantee — any tool (Glob, `ls`, Grep, other agents with no harnez)
sees only active tickets in `issues/`; hygiene is visible in the tree; matches an existing doc
convention.
**Cons**: path churn: relative links in docs/studies, commit messages and other repos break or
need rewriting; `git log -- path` needs `--follow`; promote/park flapping produces noisy diffs.
**Risks**: link rewriter misses (only exact filename refs are rewritten today); merge conflicts
when two branches move the same ticket; external references (GitHub GH<n> bodies link a path).

## 5. Proposal C — Explicit Active Set ("Now" list) + Implicit Backlog

**Mechanism**: keep files and statuses as they are; add one generated-plus-curated file
`issues/NOW.md` (or a `spec/`-schema'd `issues/active.yaml`) holding a ranked list of ≤20
ticket numbers with one-line "next step" each. Everything Open but not listed is backlog by
definition. Verbs `issues now add|rm|rank <n>`; `issues start` auto-adds; `close` auto-removes.
**Listing/search**: `harnez issues list` default = the Now list (~20 rows, ~2 KB). `is:open`
still exists for explicit wide queries; `is:now` / `is:backlog` (= open minus now) filters.
The agent-facing rule in IssueTracking.md: "read NOW, never README or `-a`".
**Duplicate flow**: gate (§2.3) searches everything; on match, `issues open <n>` if Closed, plus
`issues now add <n>` if it should be worked soon; otherwise append context and leave in backlog.
**Migration**: no file changes at all for the 247; the user (or a groom report using §2.4)
picks ~20 for NOW. Optional later: bulk-close obvious dead ones.
**Tooling/docs**: NOW file format (+schema), index renders NOW at top of README, list default,
lint (NOW entries must exist and be unresolved; size ≤ budget), IssueTracking.md(+lite), `/next`
and roadmap (234) skills read NOW. Other projects: opt-in — no NOW file → old behaviour.
**Pros**: hits the ~20 target directly, with explicit ranking (doubles as roadmap/priority
order, which P2-for-everything currently fails to express); zero migration risk; reversible.
**Cons**: second source of truth beside Status; the 800-file dir and 246 KB README remain unless
combined with the README split from A; "Open" no longer means "active", which confuses readers.
**Risks**: NOW goes stale if agents don't maintain it (needs lint + auto-add/remove hooks);
concurrent edits to one file conflict more than per-ticket status lines; agents may still
reach for `-a` habitually unless docs and skills change together.

## 6. Comparison and Recommendation

| | A Status | B Directories | C Now list |
|---|---|---|---|
| Context cost of default list | low (≤100) | lowest, tool-agnostic | lowest (~20) |
| Reaches ~20 active | needs strict grooming | needs strict grooming | by construction |
| Duplicate reach | all statuses | all dirs | all |
| Migration effort | low (batch verbs) | high (moves + links) | lowest |
| Tooling change | small | large | medium |
| Main risk | regrowth | link breakage | stale second index |

R2 recommends **A + the NOW list from C**: Backlog status for hygiene (≤100 Open), a
ranked NOW list for the ~20 actually being worked, the README split, and the dup gate. B only if
non-harnez tools reading `issues/` turn out to be a measured context cost.

## 7. Uncertainties

- Which commands agents actually run to list tickets (`-a`, README reads, skills) was not
  measured; telemetry (`harnez stats`, invocation log) could quantify the real context cost.
- Git creation dates look compressed (history rewrite/renames?); age-based criteria need checking.
- Recall of token-overlap `similar` without embeddings is unknown; validate by replaying
  known duplicate pairs from closed tickets ("Closed — duplicate of …") before relying on it.
- Whether the user wants Backlog tickets ever auto-closed (e.g. after 90 days) is a policy call.
- Impact on other harnez-managed projects (smaller trackers) is assumed small but unverified.
