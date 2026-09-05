---
title: Issues-Verb Command, Roadmap/Discovery Skills, and the Index Data-Loss Bug
---

# Issues-Verb Command, Roadmap/Discovery Skills, and the Index Data-Loss Bug

**Scope**: `cmd/harnez/issues.go`, `internal/issues/issues.go`, `internal/index/index.go`,
`commands/issue.md`, `commands/roadmap.md`, `commands/discovery.md`, tickets 232–240.

A single long session (2026-09-05) that shipped a new write-side CLI command family,
two new product-management skills, found and fixed a live data-loss bug via dogfooding,
and surfaced two more tracker-hygiene bugs during its own closing review.

---

## 1. What Shipped

### 1.1 `harnez issues <verb>` (232)

Single-call ticket status changes: `harnez issues open|start|block|close|draft <n>
[reason]`, plus `harnez issues new [title]` (moved here from `find --reserve`, see §2).
Each verb rewrites exactly the `**Status**:` line via a shared regex anchor
(`RewriteStatus`, reusing `ParseIssueFile`'s `statusLineRegex`), resyncs
`issues/README.md`, and commits — one tool call instead of the previous four-step
manual sequence (edit file, `harnez index`, `git add`, `git commit`).

Design decisions resolved via a dedicated read-only advisor subagent (not host
unilateral judgment) rather than by API-shape intuition alone:
- Commit-by-default, not opt-in.
- A closed verb set (`open`/`start`/`block`/`close`/`draft`), not free-text status.
- flock-based race safety on `issues/README.md` (matching the existing idiom in
  `internal/usage/livefetchcache.go`), not `O_EXCL`-only — `UpdateIssuesReadme` had
  *zero* locking before this ticket despite being a shared-file writer.
- `start`/`draft` accept an optional reason (validated against real corpus evidence:
  ticket 201 already used `In Progress — implementation complete; ...` in practice)
  rather than erroring on one, which was the host's initial (wrong) instinct.

### 1.2 `/roadmap` and `/discovery` skills (234, 236)

Two complementary PM-style skills, both dispatching a fresh subagent requesting a
strong model, both deriving their "value axis" fresh from the target repo's own
identity docs rather than a hardcoded formula:

- `/roadmap` — read-only against the tracker. Sequences the *existing* open backlog
  into Now/Next/Later, reconciling against a prior `docs/Roadmap.md` rather than
  overwriting it.
- `/discovery` — the complementary half: compares stated use cases (identity docs)
  against the *real* implemented surface (not the backlog — actual `--help` output or
  command registration) to find gaps neither implemented nor already tracked (checking
  *closed* tickets too, so it doesn't resurrect deliberately-rejected ideas). Reports
  candidates only; never auto-files — filing stays a human-gated `/issue` step.

Kept deliberately separate rather than merged into one skill/mode: different cadences
(discovery is occasional/exploratory, roadmap is routine) and different review needs
(discovery output needs human judgment before becoming a ticket; roadmap output never
touches the tracker at all).

### 1.3 `/issue` delegates to a subagent (235)

`commands/issue.md` no longer has the *host* run `harnez find`/draft content inline.
It composes a self-contained "Handoff Prompt" (the concrete ask verbatim, motivating
context already discussed this session, touched files/commits/tickets, explicit user
constraints) and dispatches a fresh subagent to do duplicate search + `harnez issues
new` + write + `harnez issues open --commit`. Async by default — dispatch, stay
responsive, report on completion — matching this session's general dispatch pattern
rather than treating ticket-filing as special-cased synchronous work just because it's
usually fast. This pattern was validated by immediate reuse: it's exactly how tickets
237/238 (below) got filed.

---

## 2. The `find` vs `issues` Interface Change

`harnez find issues next --reserve <title>` — a *mutating* operation living under a
command explicitly documented as read-only — was moved to `harnez issues new [title]`.
No backward-compat alias or deprecation shim was added for the removed `--reserve` flag.

This was an explicit, stated policy decision, not an oversight:

> "we can always change interfaces, as long as we have a way to sync it to all repos —
> we are still a solo developer shop until we aren't."

Clean interface cuts are acceptable here specifically because `harnez apply`/`init`
propagates changes to every consuming repo; the sync mechanism substitutes for
backward compatibility. This is repo-specific policy, not a general recommendation —
it depends entirely on harnez's own apply/init propagation existing as a safety net.

---

## 3. The Self-Applied-Doc-Copy Trap (233)

Mid-session, a doc fix for the `start`/`draft` reason-suffix note was written directly
into `docs/IssueTracking.md` (repo root) — which turned out to be a **self-applied
local copy** of `docs/practices/IssueTracking.md` (per `config.yaml`'s
`source:`/`local:` doc mapping), not the source. The edit would have been silently
discarded on the next `harnez init`/`apply`.

Caught only by accident, while investigating something unrelated (the `--reserve`
migration), when a diff between the "source" and "local copy" showed the source had
*more* content than the file that had just been hand-edited. Fixed by moving the
content to the real source and resyncing — which itself required a `go build` first,
since docs are `//go:embed`-ed at build time and the installed binary was stale enough
to silently re-clobber the fresh source edit on the first resync attempt.

**Lesson**: before editing anything under a project's `docs/` root, check whether
`config.yaml` (or the repo's own docs-layout convention) declares it a self-applied
copy of a `docs/practices|lang|other/` source. Ticket 233 (still open) proposes an
`AGENTS.md` note to prevent recurrence — filed but not yet implemented, so the trap
itself is still live for the next session that touches these docs.

---

## 4. The Index Data-Loss Bug (237) — Found by Dogfooding

While onboarding two sibling repos (`trafficsim`, `.workspace`) onto harnez-managed
docs and issue tracking, running `harnez index` against their pre-existing
`issues/README.md` files revealed that `UpdateIssuesReadme` unconditionally replaced
*everything* from the table header to EOF with harnez's own canonical bare-column
table:

```go
newContent := content[:loc[0]] + table
```

Any hand-authored prose after the table, or extra project-specific columns (Priority,
Target), were silently destroyed — not merged, not warned about, just gone. Caught
before commit only because the resulting diff was reviewed manually (`git diff`) prior
to committing; had it not been reviewed, two repos would have permanently lost
pre-existing tracker content on their very first `harnez index` run.

Fixed same-session (`f0fdcf2`): `UpdateIssuesReadme` now refuses to write (with a
clear error) when the table header line doesn't match harnez's exact expected shape,
rather than guessing how to merge an unrecognized schema. This is documented in
`docs/practices/IssueTracking.md` §4.1 as the current behavior. `UpdateDocsReadme` was
flagged as sharing the same code shape but was **not** audited or fixed in this
pass — worth checking independently before trusting it against a customized
`docs/README.md`.

Also fixed same-session (238, `7412c88`): the `issues/README.md.lock` sidecar flock
file harnez's own locking introduced (§1.1 above) wasn't gitignored by default in
newly-onboarded projects — `init` now writes the `.gitignore` entry automatically.

---

## 5. Two More Tracker Bugs, Found During This Session's Own Closing Review (239, 240)

Running the closing `harnez status`/`harnez index` check on this very session's own
work surfaced two more genuine, pre-existing tracker bugs, unrelated to anything
shipped this session:

- **239 — pipe-escaping**: ticket 233's own title contained a literal `docs/practices|
  lang|other source`. `IssuesTable` (`internal/index/index.go`) writes titles into a
  Markdown table cell with zero escaping, so the `|` characters were read back as
  extra column separators, and `ParseTrackerTable`/`Lint` mis-parsed the row —
  producing a false "unindexed ticket file" diagnostic for a ticket that was, in fact,
  correctly indexed. Worked around immediately (reworded the title) since it was
  actively breaking `harnez status`'s output; the underlying escape-on-write /
  parse-escaped-pipes-on-read bug is filed, not fixed.
- **240 — duplicate ticket numbers**: two independent pairs of tickets (179, 180) each
  share a number — a pre-existing historical collision, not something introduced this
  session. All four tickets are already `Closed`, so there's no active-work conflict,
  but it violates `IssueTracking.md`'s own invariant #4 ("no duplicate ticket numbers")
  and will keep tripping the linter on every future run until renumbered.

**Insight**: an evergreen/closing review pass is not just for writing prose — running
the project's own health-check tooling (`harnez status`, `harnez index --check`)
against its own tracker at session end surfaced two real, previously-undetected bugs
that had nothing to do with the session's actual feature work. The tooling had
presumably been reporting the pipe-escaping false-positive and the 179/180 duplicates
on every run for a while; nobody had looked closely enough at `harnez status`'s tail
output to notice.

---

## 6. Agentic-Workflow Observations

- **Advisor-dispatch for design questions, not just code review**: this session
  established (and reused four times) a pattern of dispatching a read-only advisor
  subagent specifically to resolve API/interface design questions — commit policy,
  verb-set-vs-free-text, locking idiom, reason-argument validity — rather than the
  host reasoning it out unilaterally. Each time, the advisor grounded its
  recommendation in concrete corpus evidence (an existing ticket's actual usage, an
  existing locking idiom elsewhere in the codebase) rather than abstract API taste.
  This produced better-justified decisions than host-only reasoning would have, at the
  cost of one extra dispatch round-trip per question — worth it for anything with
  lasting interface-shape consequences, not for routine implementation choices.
- **Dogfooding over pure code review for data-loss risk**: the read-only advisor
  pattern above is good for *design* questions, but the actual index data-loss bug
  (237) was found only by exercising the command against *real, pre-existing,
  differently-shaped* data (the sibling repos' own `issues/README.md` files) — no
  amount of reading `UpdateIssuesReadme`'s code in isolation would have surfaced that
  its "replace to EOF" behavior was unsafe against a README shape the author's own
  test fixtures never happened to construct. Matches `AgenticLoop.md`'s existing
  "Unit-Test-Only Confidence" anti-pattern, generalized here from hook/environment
  features to any command that mutates a file whose *existing* shape it doesn't fully
  control.
- **Leftover uncommitted work found at session-close time**: a large batch of ticket
  `## Implementation Plan` sections from an *early*-session Opus planner pass (65
  files) had never been committed — they sat as uncommitted working-tree changes
  through the entire rest of the session's other work, invisible unless `git status`
  was checked directly (nothing in the later work touched those files, so nothing
  surfaced the gap). Caught and committed only as part of this closing review's
  routine `git status` check before starting the documentation pass itself.
  **Lesson**: a large parallel-dispatch batch's output should be committed
  immediately after review, not left implicitly staged for "whenever the next commit
  happens to touch nearby files."

---

## 7. Open Threads

- Ticket 233 (repo-local-vs-source doc-edit warning) — filed, not implemented; the
  trap it documents is still live.
- Ticket 237's sibling suspect, `UpdateDocsReadme` — flagged as sharing the same
  "replace to EOF" code shape, not audited.
- Tickets 239 (pipe-escaping) and 240 (duplicate numbers 179/180) — filed, not fixed.
- Tickets 230/231 (the 224/166 ticket splits) — filed, not implemented; no request yet
  to pick them up.
