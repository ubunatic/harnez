# Product-Manager-Style Roadmap Creation/Update

Synthesize (or reconcile) a roadmap document from the open issue backlog, acting as a product
manager whose sequencing decisions are justified by value delivered to the product's actual use
cases — not ticket volume, not "what's easiest," not a fixed priority-field sort.

Reference Ticket: `issues/234-add-a-roadmap-skill-product-manager-style-roadmap-creation-update-from-open-issues.md`
Reference Precedent: `docs/Roadmap.md` in this repo (hand-built example of the exact output shape
this skill should produce), `commands/harnez-sync.md` (closest structural precedent — autonomous,
dispatches a fresh subagent, drives existing CLI commands rather than reimplementing logic inline).

---

## Invocation Syntax

- `/roadmap` — target the current repo (`.`).
- `/roadmap <repo>` — target `<repo>` explicitly, matching other skills' `-d <repo>` convention.

---

## Autonomous Workflow

This is a multi-minute synthesis task, not a quick lookup. Dispatch a fresh subagent to run the
full workflow below to completion, requesting a strong/capable model for the dispatch (e.g. Opus,
or the strongest model this harness can spawn — the synthesis step in Step 3 is where model
quality matters most; a fast/cheap default model is not a good fit here). Keep the host
orchestrator responsive — report the dispatch and don't block the main chat on the subagent's
result unless the user explicitly asked to wait.

### Step 1 — Check open issues

- Run `harnez find -d <repo> issues -a status:open -I` (or `harnez find`) to get the backlog overview card and inspect it with `view_file` for an immediate high-level visual survey. Use `-r` or `--json` when scripting programmatic extractions.
- Read each open ticket's file (preferring `harnez issues show -d <repo> <n> -I` or targeted range reads to avoid text context inflation). Most tickets carry an appended `## Implementation Plan` section
  from a prior planning pass — if present, use it for scope, dependencies, and blockers rather
  than re-deriving them from scratch. Fall back to the ticket's own problem statement and
  acceptance criteria only when no plan section exists.
- Skim root-level `docs/*.md` briefly for architectural context if useful (e.g. using `harnez read -I <doc>` to inspect visual context cards), but don't do deep
  whole-file ingestion — Context Discipline norms apply (see `docs/AgenticLoop.md` Invariant 6):
  prefer targeted greps, image cards, and range-bounded reads over bulk document ingestion.

### Step 2 — Check the current roadmap

- Look for an existing roadmap document before assuming a fixed path. Check the repo's own
  docs-layout convention first: read its `AGENTS.md`/`CLAUDE.md`/`README.md` for a stated
  "Docs Layout" or similar section that names where the repo's own evergreen docs live. If the
  repo documents such a convention, follow it (e.g. a repo whose docs say root-level `docs/*.md`
  holds that codebase's own evergreen docs, as opposed to copyable docs kept in subdirectories,
  should get its roadmap at `docs/Roadmap.md`). If nothing is indicated, default to
  `docs/Roadmap.md` at the project root.
- If a roadmap document already exists at that path, this is an **update**, not a fresh
  generation. Read it in full and reconcile against it rather than overwriting it wholesale:
  - Tickets that shipped since the last pass move out of the roadmap, with a note (or move to a
    "Shipped" section) rather than silently vanishing.
  - Tickets that got reprioritized move buckets, and the reasoning for the move should be visible
    in the document, not just silently overwritten.
  - Tickets closed or superseded since the last pass are removed or marked accordingly.
  - Preserve sections/framing that are still accurate; only rewrite what the current backlog state
    actually invalidates.
- If no roadmap document exists yet, this is a fresh generation — proceed to Step 3 to build one
  from scratch.

### Step 3 — Create or update the roadmap

Act as a product manager. Before grouping or sequencing anything, establish what this specific
product's value axis is by reading the repo's own identity docs — `README.md`, `AGENTS.md`,
`CONTEXT.md` (whichever exist) — for what the product is actually for and how it's actually used
day to day. This judgment must be derived fresh from the target repo's own docs every time, never
assumed or hardcoded in this skill file: a tool built around a live monitoring dashboard has a
completely different value axis than a tool built around transcription accuracy, and this skill
must produce the right roadmap for either without being told which kind of repo it's running in.

With that value axis established:

- Group open tickets into coherent themes inferred from their actual content — not a fixed
  taxonomy imposed in advance. Themes should emerge from what the backlog actually contains.
- Propose a Now/Next/Later-style sequencing (or reconcile the existing one) based on: dependencies
  between tickets, stated scope size, and — most importantly — what most increases the product's
  value along the axis established above. State the rationale for each bucket placement; a bare
  priority-field sort is not sufficient justification.
- Call out tickets that look like candidates to close or deprioritize — superseded by other work,
  or genuinely blocked on an external decision — in a dedicated section (e.g. "Close / Park")
  rather than forcing them into a Now/Next/Later bucket they don't belong in.
- Write (or update) the roadmap document at the path determined in Step 2.

### Non-Goals / Constraints

- **Read-only against the issue tracker.** Never edit any `issues/*.md` ticket file, never touch
  `issues/README.md`, never run `harnez index` or `harnez issues <verb>`. The only file this skill
  writes is the roadmap document itself.
- Not a replacement for the issue tracker, and not a scheduling or assignment tool — it produces a
  synthesis/communication artifact, not new task metadata.
- No fixed scoring rubric or formula for "value to use cases" — that judgment is made per repo by
  the dispatched subagent, informed by that repo's own docs, not computed from a fixed algorithm.

### Final Report

Report back to the operator: the roadmap path written or updated, a one-line summary of the
sequencing rationale, and — on an update — what moved (shipped/closed/reprioritized) since the
prior version. Name moved tickets with a short label, not a bare number ("289 (go.work testdata)").
