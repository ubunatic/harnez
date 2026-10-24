# 910 — Epic: repo setup, tracker, tests and agent guidance

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 907, 897, 864, 855, 867, 353, 898

---

/goal Complete the tickets below with `/lean-sprint` runs; each closes only when its own acceptance holds, a
shortfall gets its own ticket. Stop and report when a worker loops, a blocker cannot be solved, or a ticket needs
an owner decision. Circuit breaker: on no progress, clear the goal and write a failure report here.

## Tasks

Web fanout skipped: internal harnez tooling, and neus is the owner's own tool. Code fanout: haiku:low searcher
`e910-search`, 2026-10-24.

| # | Ticket | Pri | What | Group | Developer | Reviewer | Status |
|---|---|---|---|---|---|---|---|
| 1 | 353 | P3 | Claude-only CxxxE.md rule | — | — | host | Closed: already delivered by `harnez hook claude-instructions` |
| 2 | 898 | P3 | `gokay check` line in `docs/lang/Go.md` + root copy | A | haiku:high | host diff | Closed (68b121d6) |
| 3 | 867 | P3 | `issues close` refuses without a code commit; `--no-code` override | A | haiku:high | host diff | Closed (d17723da) |
| 4 | 907 | P2 | Audit copy fixtures, Canary.md three rules | B | luna:high | host diff | Open |
| 5 | 864 | P2 | Session push consent in Git doc; release pushes every dev remote | C | sol:med | host diff + tests | Open |
| 6 | 897 | P2 | `harnez install neus`; server indexing via the neus binary | D | sol:med | opus reviewer | Open |
| 7 | 855 | P3 | `harnez init` feature wizard | E | sol:med | opus reviewer | Open |
| 8 | — | — | `/evergreen`, `/wrap` | — | host | — | Open |

## Decision

The owner delegated with `/goal complete the epic`; the host decided:

- 907: searcher found no fixture that copies repo code in the five listed files; candidates
  `internal/claude/lite_doc_structural_gate_test.go`, `cmd/harnez/telemetry_writer_boundary_test.go`. The dev
  audits these, applies the verdicts, and replaces the Canary.md section with the three rules.
- 864: one push confirmation covers the session; "push" means `main` and tags to both dev remotes, ff-only,
  stop on divergence. `harnez release` pushes to every configured dev remote (remote names from config/spec).
- 897: neus has no public Go API (search code is under `internal/`), so harnez shells out to the `neus` binary,
  as the finder already does. `harnez install neus` runs `go install` + `neus install`. Server indexing is
  opt-in, and skips when a neus indexer service is active (neus issue 057), so the two never scan concurrently.
- 867: refuse by default, `--no-code` overrides for tickets that needed no code (docs/tickets only).
- 855: TTY prompt per feature with one-line explanations; no TTY or `-y` keeps today's detection + config path.
