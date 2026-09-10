# Roadmap Now Sequential Sprint Retrospective

**Scope:** sequential execution of the roadmap's high-value Now items, with one reusable
Astra-low advisory session.

## Delivered

- Verified the existing usage-watch fixes for 290, 210, 201, 139, 105, and 262, then closed
  their tracker entries with durable resolution notes.
- Added bounded `find issues` output (`-n`/`--limit`, `--all`, and bare-list defaults) for 217.
- Fixed cached issue-index diagnostics for unrelated untracked tickets (292).
- Fixed GoRelease auto-detection and the consumer-safe AgenticLoop reference (299).
- Documented closure traceability (126), canary-first prototyping (263), and Go terminal display
  width invariants (166 Part A).
- Extended collector diagnostics with structured duration/error records and basic bounded scrolling
  while preserving the existing progress callback API (255 remains open for full escape-sequence
  decoding and end-to-end interaction coverage).

## Effectiveness and agentic coding learnings

- A single reusable read-only advisor was sufficient for a broad sequential audit. It quickly
  separated already-shipped roadmap claims from actual implementation gaps, avoiding redundant
  feature work.
- Frequent stable commits and immediate tracker commits made the long sprint recoverable and left
  each ticket's status explainable.
- The roadmap was stale in several Now rows: implementation had landed, but ticket closure had not.
  Future roadmap passes should distinguish “implementation complete” from “tracker closure pending”.
- A broad package test produced too much fixture output to expose its failure clearly. Narrowing to
  the named failing subtest was materially faster and safer.
- Config insertion near dependency metadata caused an existing dependency line to move to the wrong
  document. Configuration edits need a targeted post-patch context check before running the suite.

## Architecture changes

- `find issues` now keeps ranking in `internal/find` and applies presentation limits in the CLI:
  ranked text searches take the best results, while filter-only/bare listings take the newest.
- `internal/usage` now exposes an additive `FetchDiagnostic` callback; legacy progress consumers
  remain unchanged, and the watch overlay renders sanitized error text and fetch duration.
- Copyable documentation now includes `PrototypingFeatures`, linked through config and the project
  docs layout, while the local-LLM profile remains separately blocked as intended.

## What I would do differently

- Start with a smaller, explicit roadmap ticket set generated from the Now bucket and its current
  tracker statuses, rather than initially treating every listed Now row as implementation work.
- Add the arrow/PageUp/PageDown input decoder and its PTY-level test before touching the 255
  completion boundary; the current `j/k/u/d` navigation is useful but not the full contract.
- Run the final independent review phase explicitly before the user asks to end the sprint.

## Harness/process proposals (not applied)

- Add a roadmap command/report that marks rows as `shipped`, `closure-pending`, or `open` from
  tracker status and recent implementation notes.
- Add a small config validation check that reports dependency lines whose surrounding document key
  changed unexpectedly.
- Give the watch input layer a shared escape-sequence decoder so overlays can consistently support
  arrows and paging rather than byte-at-a-time approximations.
