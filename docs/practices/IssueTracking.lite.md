<!-- harnez:variant=lite -->
# Issue Tracking Rules (Lite)

In-repository tracker in `issues/`, one file per ticket, consistent priorities, metadata
headers, and lifecycle rules. Full doc carries the rationale and worked examples.

## 1. Priority schema — scheduling urgency

| P | Level | Trigger | SLA |
|---|---|---|---|
| **P0** | Critical | Blocker, data loss, security vuln, broken build, critical regression violating core invariants. Stop the line. | Immediate; precedes all other tasks |
| **P1** | High | Core functionality broken, major workflow impediment, key API regression, high-urgency milestone deliverable. No acceptable workaround. | Current sprint / next release cycle |
| **P2** | Medium | Normal feature, standard bug fix, perf optimization, UX polish, refactor without active blockage. Workaround may exist. | Standard backlog; routine planning |
| **P3** | Low | Minor cosmetic glitch, typo, nice-to-have, speculative idea, non-urgent doc improvement. | Opportunistic, when touching related code |

## 2. Priority ≠ Severity

- **Severity** = technical impact/damage (how badly the system is broken).
- **Priority** = scheduling urgency (how fast it must be fixed).

Typo in the homepage brand header: Low Severity (cosmetic), **P1** (reputational urgency).
Memory leak in an obscure deprecated unadvertised CLI flag nobody uses: Critical Severity
(crash/resource exhaustion), **P3** (low operational risk).

## 3. Ticket file + metadata

Path: `issues/NNN-kebab-case-title.md` (e.g. `issues/042-standardized-issue-priority-schema.md`).

### 3.1 Allocating & reserving numbers — never ad hoc shell

DON'T `ls | grep | sort | tail`. Query (read-only) and reserve (write) are separate commands:

```bash
harnez find issues next           # next free number = max(allocated)+1 over issues/*.md
                                  # + issues/archive/*.md. Reserves nothing.
harnez find issues next --json    # {"number":"195","reserved":false}
harnez issues new "Ticket Title"  # atomically allocates + creates placeholder ticket
harnez issues new                 # same, untitled -> issues/NNN-reserved.md, status Draft
```

`issues new` uses `O_CREATE|O_EXCL` to prevent collisions between concurrent agents and prints
`NNN<TAB>issues/<reserved-filename>.md`. DO write the real content to that printed path — a
hand-derived slug can diverge from the reserved filename and orphan the placeholder (issue
202). `--json` adds `"reserved":true` plus `file`/`path`. `new` never commits.

KNOWN GAP — cross-clone collisions survive `O_CREATE|O_EXCL`: the atomic reservation only
guards writers sharing one working tree, not a number reserved in another clone/session that
hasn't been pushed. Surfaces later as a `git pull`/rebase conflict on the ticket file and on
`issues/README.md` (happened twice: issue 240's duplicate 179/180; a local 266 colliding with
remote 266/267). Manual recovery until `harnez issues mv` (issue 269) ships: `git mv` the
losing ticket to the next free number, fix its in-file `# NNN — ...` header to match, resolve
the `issues/README.md` conflict by taking either side (`git checkout --theirs`) and then
regenerate authoritatively with `harnez index` rather than hand-merging — `harnez index` does
not always clear stray `<<<<<<<`/`=======`/`>>>>>>>` lines, so grep for conflict markers after.

### 3.2 Updating ticket status — `harnez issues <verb>`
- `harnez issues open|start|block|close|done|draft <n> [reason]` — rewrite Status, resync README, commit.
- `done` is an alias for `close`.
- `harnez issues mv <old> [new]` — renumber ticket, rename file, rewrite `# NNN — ...` header, resync.
- `harnez issues list [filter]` — list matching tickets (default `is:open`).

### 3.3 Required metadata block at the top of every ticket

```markdown
# NNN — Title of the Issue

**Status**: Open | In Progress | Blocked — <reason> | Closed — <resolution> | Draft
**Priority**: P0 (Critical) | P1 (High) | P2 (Medium) | P3 (Low)
**Severity**: Critical | Major | Moderate | Minor
**Category**: Bug | Feature | Architecture | Documentation | Performance | Refactor | Agentic Ergonomics
**Related**: [Doc / Ticket / Commit references]

---

## 1. Problem & Motivation
...

## 2. Technical Specification / Findings
...

## 3. Implementation & Verification Plan
...
```

### 3.3 Allowed values

- **Status**
  - `Open` — unresolved, ready to work on; optional `— <note>` suffix.
  - `In Progress` — actively worked this session; optional `— <note>` suffix (e.g. `In Progress
    — implementation complete; tracker closure awaits ...`).
  - `Blocked — <reason>` — waiting on upstream/external; reason REQUIRED.
  - `Closed — <resolution>` — completed and verified with tests (`Closed — resolved`,
    `Closed — invalid`). DON'T record the closing commit hash: a hash is content-addressed and
    unknowable by the commit that writes it, so self-reference needs a fixup commit. Record the
    resolution in words; trace via `git log --oneline -- issues/NNN-*.md`. A hash is optional
    only when deliberately pointing at an *earlier* commit.
  - `Draft` — tentative proposal or placeholder; optional `— <note>` suffix.
- **Priority**: `P0 (Critical)`, `P1 (High)`, `P2 (Medium)`, `P3 (Low)`
- **Severity**: `Critical`, `Major`, `Moderate`, `Minor`
- **Category**: `Bug`, `Feature`, `Architecture`, `Documentation`, `Performance`, `Refactor`,
  `Agentic Ergonomics`, `Infrastructure`

## 4. Index & archive conventions

### 4.1 `issues/README.md` index table

DO run `harnez index` (issue 148) to regenerate the table from `issues/*.md` +
`issues/archive/*.md` metadata; DON'T hand-edit rows — the next run overwrites them, fix the
source ticket/study file instead. It is idempotent, prints a unified diff of exactly what would
change (so running it in an agent session surfaces actionable drift without a separate diff
step), and `--check` exits 1 on drift without writing, for CI/pre-commit. Only the consecutive
table lines starting at the exact `| # | File | Title | Status |` header are managed — prose
before/after is preserved verbatim. A customized header (added Priority/Target column) is
REFUSED without writing, because harnez can't regenerate project-specific columns; reconcile
that schema manually first. `harnez index` also regenerates the studies index where that
convention exists; otherwise the issues index updates independently.

```markdown
# Issues

| # | File | Title | Status |
|---|------|-------|--------|
| 001 | [archive/001-diff-clean-wrong-path.md](archive/001-diff-clean-wrong-path.md) | diff and clean operate on wrong file | Closed — resolved |
| 042 | [042-standardized-issue-priority-schema.md](042-standardized-issue-priority-schema.md) | Standardized issue priority schema | Closed |
```

`harnez status` lints the tracker: (1) every ticket on disk is indexed; (2) every table link
points to a valid file (across `issues/` and `issues/archive/`); (3) file status matches index
status; (4) no duplicate ticket numbers.

### 4.2 Archiving

Once closed and verified, move the ticket to `issues/archive/NNN-kebab-case.md` to keep active
`issues/` clean, and update its markdown link in `issues/README.md`.

## 5. Lifecycle invariants

1. **Test verification before closure** — never mark `Closed` without running the suite
   (`go test ./...`, `make check`) and confirming assertion rigor.
2. **Atomic index sync** — status changes in the file update `issues/README.md` immediately.
3. **Immediate tracker commit** — commit ticket file + synced index in their own small commit
   right away; DON'T batch tracker metadata with unrelated code or defer it to a later
   feature-work checkpoint.
4. **Traceability** — link study notes, retrospectives, ADRs, commits in `**Related**:`, using
   the project's existing durable documentation location.
5. **Closing is part of done** — noting a ticket `In Progress` is disciplined for free; closing
   is not, because nothing forces the last step. `smarthome` shipped 112 commits in three days
   with excellent open/progress hygiene and still left four tickets reading `In Progress` for
   work that was demonstrably shipped and live-verified. A session ending on a green build and
   a commit is NOT done until every ticket it touched has its `Status` flipped and `harnez
   index` has run. Make "did I close what I finished?" an explicit end-of-session check.
