# 356 — Extract Anti-Patterns section from AgenticLoop.md into its own copyable doc

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Templates / Docs / Token Efficiency
**Related**: [Issue 359](359-pilot-agenticloop-lite-md-behavioral-canary-gate.md) (lower-blast-radius
alternative — a lite variant of the whole doc may make this extraction unnecessary; see that
ticket's cross-link back here), [Issue 354](354-collapse-duplicated-instruction-blocks-across-claude-md-agents-md-templates-into-single-source-pointer-pattern.md) (prompted this assessment), [Issue 323](323-docs-lang-bash-md-hard-references-docs-practices-agenticloop-md-instead-of-docs-agenticloop-md-alias.md) (path-aliasing fragility — do this first), [Issue 249](249-copied-practice-docs-retain-dangling-and-inapplicable-dependencies.md) (per-project doc drift — this multiplies that surface), [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md)

---

## 1. Problem & Motivation

`docs/practices/AgenticLoop.md` is 26KB / ~9.1k tokens installed. Assessed for a full
4-way split (Invariants / Sprint Loop / Friction Reporting / Anti-Patterns) after issue
354; a full split was **not recommended** — see the assessment below — but §6 ("Practical
Recipes & Anti-Patterns", lines 228-268) stood out as the one section worth extracting on
its own:

- It's the largest, most heterogeneous section (case studies, long war-story bullets) —
  a materially different consumption pattern (reference/lookup material) than the
  Invariants/Phases sections (loaded every sprint session).
- Nothing references it by exact anchor name from active code/config/commands. The other
  sections are: Invariants referenced by name ("Invariant 3/6/7") from `harnez/AGENTS.md`,
  `config.yaml`, `docs/templates/AGENTS.md`, `docs/README.md`,
  `internal/usage/miclive.go`; Phases referenced by name ("Phase 2/3/5") from
  `commands/sprint.md` and `commands/lean-sprint.md`. Extracting Anti-Patterns needs no
  rewrite of those pointers.

## 2. Assessment Findings (full split rejected, scoped extraction proposed)

Full 4-way split was rejected because:
- `AgenticLoop.md` is a registered `config.yaml` docs-registry entry
  (`source`/`target`/`local`/`DependsOn: issue-tracking`), asserted by exact filename in
  three Go tests (`internal/claude/docs_test.go`, `integration_test.go`, `init_test.go`).
- Issue 323 already flags the `docs/practices/AgenticLoop.md` vs. installed
  `docs/AgenticLoop.md` alias as fragile; a full split would multiply that fragile
  alias-pairing N times.
- Issue 249 already flags per-project copied-doc drift/dangling dependencies; more files
  means more drift surface across every project that installed this doc.
- Measured token payoff of a full split is theoretical, not observed: every current
  consumer that references an invariant or phase already has the whole doc installed
  anyway (a project using `/sprint` needs both invariants and phases), so per-session
  savings from separate installs are unlikely to materialize in practice.

A single-section extraction (Anti-Patterns only) avoids the above: no active pointer
rewrites needed, and it's the section most plausibly *not* needed by every consumer of
the rest of the doc (a project running the lean-sprint fast path may never need the full
case-study anti-pattern list).

## 3. Proposed Fix

1. Do this **after** issue 323 lands (fix the path-aliasing mechanism first — don't build
   a second alias pair on top of a known-fragile one).
2. Create `docs/practices/AgenticLoopAntiPatterns.md` (or fold into a more fitting home if
   one exists by then) containing §6's content verbatim, marked `<!-- harnez:bundled -->`.
3. Register it in `config.yaml`'s docs registry, `DependsOn: agentic-loop` (it references
   Invariant 3 by name once, in the "Chat-Visible Empty Polling" entry).
4. Remove §6 from `docs/practices/AgenticLoop.md`, leaving a one-line pointer:
   "See `@docs/AgenticLoopAntiPatterns.md` for the anti-pattern case-study list."
5. Update the three Go tests that assert `AgenticLoop.md` content/filename only if they
   assert anything from the removed §6 (check `docs_test.go` fixtures first — likely no
   change needed since they check filename existence and dependency wiring, not section
   content).
6. Add the new file to `docs/README.md`'s doc table (regenerated via `harnez index`, not
   hand-edited).

## 4. Acceptance Criteria

- [ ] Issue 323 closed first, or this ticket explicitly re-scoped if 323 changes the
      alias mechanism in a way that changes this plan.
- [ ] `docs/practices/AgenticLoopAntiPatterns.md` exists, registered in `config.yaml`,
      installs via `harnez init --docs agentic-loop` (or its own flag — decide which at
      implementation time) alongside/independent of the parent doc.
- [ ] `docs/practices/AgenticLoop.md` no longer contains the Anti-Patterns list; the
      one-line pointer resolves correctly for a project that installed both docs.
- [ ] `go test ./...` and `scripts/smoke-test.sh` pass; `harnez apply`/`init` remain
      idempotent on re-run.
- [ ] Net byte/token reduction measured for a project that installs `agentic-loop` but
      not the anti-patterns doc (if that's the chosen opt-in granularity).
