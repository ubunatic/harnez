# Ticket hygiene: three file-based approaches (sol)

Date: 2026-10-21. Research for issue 842; repository inspection only, no implementation.
Provider/session: OpenAI Codex, researcher `sol`; no child agents or other researchers' reports used.
All commands/fields described below as proposals are new interfaces, not existing capabilities.

## Observed baseline

- Ticket 842 records approximately 247 active tickets. This inspection returned 249: 238 leading `Open`, seven `Blocked`, four `In Progress`; annotated Open is included in 238.
- `harnez issues list` defaults to `is:open`; both it and `find issues` default to ten results. Filter-only listings choose newest IDs, not highest priority (`cmd/harnez/find.go:393-428`, `cmd/harnez/issues.go:423-439`).
- `.harnez/rules/Tools.md:38` and `Issues.md:5` teach `-a status:open`, defeating that output bound. Changing only the CLI limit will not solve routine context cost.
- `internal/issues/issues.go:387-438` scans root tickets and exactly `archive/`; other directories are skipped. It reads full files internally. Explicit-only must mean excluded from routine results/context, while deliberate duplicate checks can inspect every tier internally.
- `internal/find/search.go:28-108` filters and ranks title/body text. Open includes Open/In Progress/Blocked, irrespective of directory. Metadata is not searchable text. Fuzzy terms combine with AND; synonyms and conceptual duplicates can be missed.
- `internal/index/index.go:67-114` generates one all-ticket README table and four-category telemetry. Moving files into archive alone neither shortens that table nor excludes unresolved tickets from open queries.
- `issues new` reserves a Draft immediately and refreshes README, with no content-duplicate check (`cmd/harnez/issues.go:675-709`). `issues mv` renumbers; it is not a tier-move operation.
- `docs/IssueTracking.md:58-70` allows archive moves only for verified closed tickets. Neither Backlog nor Archived is a defined lifecycle state. Full/lite sources are `docs/practices/IssueTracking*.md`.

## Shared requirements and grooming evidence

- Distinguish an active work commitment from unresolved inventory. Aim for 20 active, warn above 100; never silently discard tickets to meet a cap. In-progress and urgent regressions remain visible.
- Sample 005 describes a concrete permission-reconciliation bug; retain only after checking current code/reproduction and urgency. Its old ID alone is not a demotion criterion.
- Sample 007 mixes completed and remaining test gaps and names removed helpers; refresh remaining scope and overlap with 005 before scheduling it.
- Sample 010's header says blocked, while its later implementation plan says the blocker landed. Refresh status from evidence rather than trusting header or age.
- Sample 013 is a speculative feature without a recorded commitment: backlog candidate. Sample 073 explicitly says deferred/P3/security decision pending: backlog or dormant archive candidate, not completed work.
- Sample 034 has a later architecture correction and superseded implementation assumptions: reconcile with linked successor work. Blocked Dot8 results 440/441/444/447/459 form a paused dependency chain; demote together unless currently sponsored.
- These are grooming candidates, not verified closure decisions. Do not close merely because old, missing metadata, or lacking recent activity. Archive unresolved work without implying tests passed.
- Migration: snapshot IDs, paths, statuses and counts; review in bounded batches; preserve genuine current work; first reduce ~247 to <=100, then select ~20 with user/product-owner input. At fixed inventory that parks >=147, ideally ~227 (249 now implies 149/~229). Record reasons and revival triggers.
- Every approach needs a short active README plus explicit links to separate generated backlog/history indexes. Lint must validate completeness across all generated indexes, not require every file in the active table. `--all` expands result count only, never hidden-tier scope.
- Shared duplicate flow: before reservation, run an explicit all-tier candidate search, including closed, dormant, draft and both ID namespaces; show only a bounded shortlist with ID/title/tier and match evidence.
- Use weighted independent title/problem terms or a dedicated candidate scorer, not the entire proposed title as an AND query. No external embeddings are necessary initially; measure recall with known duplicate pairs.
- Caller confirms semantic identity from ticket evidence: reuse active ID; promote backlog; restore dormant archive; reopen closed regression only when the original problem recurs. Preserve prior resolution/history and append new evidence. Distinct related work gets a new ID linked to the old one.
- Proposed `issues new` duplicate gate should require an explicit existing-ID choice or explicit distinct-ticket override on candidates, and support structured noninteractive outcomes. Never auto-reopen from fuzzy similarity alone; recheck under tracker lock before allocating. Titleless reservations need explicit bypass plus a subsequent content check.
- Remote allocation must run the gate before `gh issue create`; a reused local or GH ticket must not allocate another remote ID. Preserve exclusive reservation and collision checks across all tiers.

## Proposal 1: lifecycle states encode backlog and dormant archive

- Mechanism: add `Backlog — reason` and `Archived — reason` statuses. Keep unresolved files at their stable root paths; archive is logical. Existing physically archived closed files remain supported. Archived means dormant, not resolved; Closed retains its verification meaning.
- Listing/search: ordinary list/find uses active statuses only; explicit `status:backlog`, `status:archived`, or proposed `--scope all` reaches hidden states. Closed history also requires explicit scope. `show <ID>` remains direct access across tiers. Active output is bounded to ~20 with total count and priority-first ordering.
- Duplicate flow: dedicated all-tier candidate search bypasses default scope. Promote Backlog via `open`, restore Archived via `open`, reopen Closed with recorded recurrence, preserving the same filename/ID.
- Migration: after shared grooming, write Backlog for valid unscheduled items and Archived only for intentionally dormant/superseded ideas with reasons; leave ~20 active. Do not use Draft as a substitute for mature backlog.
- Tooling/docs: extend status parser, query enum, verbs/completion, JSON, telemetry and lint; teach new meanings in full/lite IssueTracking sources and managed discovery rules. Index partitions by lifecycle and writes hidden-tier indexes explicitly linked from README.
- Pros: stable links, simple one-field policy, easy grep/manual inspection, no mass path rewrites; modest implementation surface compared with directory moves.
- Cons: scheduling and execution stage share one field; parking a Blocked item loses its structured Blocked state unless copied into the reason/history. More lifecycle values propagate to consumers.
- Risks: current substring-based canonicalization can misclassify a new status whose reason contains old status words; add explicit leading-state recognition before fallback. Older binaries see new states as Unknown or misclassify them; document minimum compatible version.

## Proposal 2: independent queue metadata, stable ticket paths

- Mechanism: add `**Queue**: Active | Backlog | Archive`, retaining Open/In Progress/Blocked/Closed/Draft lifecycle. Logical archive keeps files in place; physical archive remains optional for existing closed tickets. Queue is the scheduling authority; README is derived, never a second source of truth.
- Listing/search: default scope is Queue Active plus actionable lifecycle; explicit `queue:backlog`, `queue:archive`, or `--scope all` exposes inventory. `status:open` retains its unresolved meaning within selected scope; all unresolved work requires `--scope all status:open`. Missing Queue has documented legacy semantics until explicit migration.
- Duplicate flow: full-scope candidate search ignores Queue. Promote by setting Queue Active; retain a valid Blocked stage, or reopen Closed with evidence. A known blocked recurrence can be promoted to Backlog instead if it is still unactionable, with an explicit outcome.
- Migration: map current active inventory to Queue Active provisionally, then assign Backlog/Archive from grooming. Preserve lifecycle and blocker reasons verbatim. Closed/Draft legacy records default outside routine active results; no mandatory rewrite of every old closed ticket.
- Tooling/docs: parse/validate Queue, add queue filters and tier mutation verbs, include queue in JSON/results, update index/lint and add queue-aware counts without redefining historical open telemetry. Queue transitions plus index updates use the existing transaction/commit conventions.
- Docs/install: update both IssueTracking variants, root lite copy and managed discovery instructions; add versioned compatibility guidance for consumer projects. Explicitly exempt backlog/archive transitions from the rule implying every touched ticket must close.
- Pros: accurate separation of commitment and progress; parked blockers retain meaning; stable links and existing statuses; clearest reporting of 20 active versus all unresolved work.
- Cons: extra header and two-dimensional invariants; older binaries ignore Queue and can print every unresolved item. Manual edits can produce nonsensical Active+Closed combinations unless lint handles them.
- Risks: ambiguous missing-field defaults could keep legacy projects noisy or hide work. Require explicit migration mode/report before changing their discovery defaults; reject `start` on a hidden ticket unless it atomically promotes. Define whether promotion at the cap needs an explicit replacement or a reported exception.

## Proposal 3: directory tiers with an active root and separate indexes

- Mechanism: root `issues/*.md` holds active tickets; `issues/backlog/*.md` holds valid unscheduled work; existing `issues/archive/*.md` holds dormant and closed history. Lifecycle stays unchanged; archive means storage tier, not completion. Directory is the sole tier authority.
- Listing/search: default discovery scans/returns active root only. Explicit `--scope backlog|archive|all` searches named tiers; exact-ID show and duplicate/number-allocation/lint always see all supported directories. Short root README links separate backlog/archive READMEs without inlining rows.
- Duplicate flow: all-tier search finds both dormant Open and Closed tickets. On confirmed recurrence, atomically move the same ID to root and reopen only if Closed; retain valid blocker state. Rewrite known path references and ticket-relative links; no renumbering.
- Migration: after shared grooming, move ~227 unscheduled tickets for the ideal 20-ticket target; move closed root files to archive separately. Generate a path mapping, validate incoming/outgoing links and counts, and keep ID-based CLI access stable. Dry-run/review each batch before moving.
- Tooling/docs: extend ScanFS's explicit directory allowlist to backlog, add tier-aware scanning and move verbs distinct from renumbering; make lock/number reservation/checks global. Update all index/lint assumptions and README path resolution; add rollback for partial move/index failures.
- Docs/install: redefine archive as storage, distinguish dormant Open from Closed, and teach explicit scopes in both IssueTracking variants and managed rules. Deploy compatible scanners before moving any consumer project's tickets; a legacy binary currently skips backlog entirely.
- Pros: strong visible separation, lean root even for shell/editor browsing; existing archive concept provides a recognizable starting point; no new per-ticket metadata.
- Cons: largest migration cost; filename links and relative body links change; directory semantics must be respected by every client. Global duplicate checks still read hidden files internally.
- Risks: old binaries can overlook backlog IDs and reuse numbers; stale links or concurrent moves can strand references. All entry points must share tier discovery, and moves require the same lock as allocation. Moving alone does not solve context cost unless default search/index policies also change.

## Implementation sequence, validation and uncertainties

- Recommendation: prefer Proposal 2 for preserved lifecycle and stable paths; Proposal 1 is the smaller alternative if extra metadata is unwelcome. Proposal 3 is strongest for physical separation but has the highest compatibility/link risk.
- Sequence: settle tier semantics and hidden-search exception; implement shared scope/duplicate contracts and index lint; verify compatibility; produce a grooming dry-run; apply reviewed batches; update distributed guidance; then measure active counts and routine output size.
- Required checks: default list/search never leak hidden rows; `--all` preserves scope; explicit duplicate search finds archived body-only matches; same-ID revival preserves old evidence; closed versus unresolved archive semantics remain distinct; local/GH collisions and concurrent reservation remain safe.
- Also verify index idempotency/completeness across partitions, telemetry continuity, structured truncation metadata, and consumer migration. Directory approach additionally needs incoming/outgoing relative-link and rollback tests.
- Uncertain: whether archive should mean only completed history or also abandoned unresolved ideas; whether ~20 includes short-lived blockers and Draft intake; who owns periodic grooming and cap exceptions. These require a product decision.
- Uncertain: current duplicate recall on real examples; fuzzy typo matching does not establish semantic identity. A guarantee of zero conceptual duplicates is not supported by current search; evaluate a curated set before choosing thresholds or claiming prevention.
- Uncertain: how many sampled old issues are still reproducible; no feature verification or tests were run. Counts can change while other sessions work; re-snapshot before migration.
- No code, tickets, indexes or commits changed by this research. Only this report was written; no install or index regeneration was run because the task restricts writes to this path.
