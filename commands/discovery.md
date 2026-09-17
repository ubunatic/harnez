# Product Discovery — Gap Analysis Between Use Cases and Implementation

Identify capabilities a product's stated use cases would reasonably require that neither the
current implementation nor the issue tracker already covers — the "what should exist but nobody
has thought of yet" half of the loop that `/roadmap` (which only sequences tickets that already
exist) deliberately leaves untouched.

Reference Ticket: `issues/236-add-a-discovery-skill-gap-analysis-between-product-use-cases-and-current-implementation.md`
Reference Precedent: `commands/roadmap.md` (structural precedent — same "read the repo's own
identity docs to establish its value axis" step reused here, same dispatch-a-fresh-subagent shape).

---

## Invocation Syntax

- `/discovery` — target the current repo (`.`).
- `/discovery <repo>` — target `<repo>` explicitly, matching other skills' `-d <repo>` convention.

---

## Autonomous Workflow

This is a multi-minute analytical task, not a quick lookup. Dispatch a fresh subagent to run the
full workflow below to completion, requesting a strong/capable model for the dispatch (e.g. Opus,
or the strongest model this harness can spawn — Step 3's judgment calls are where model quality
matters most; a fast/cheap default model is not a good fit here). Keep the host orchestrator
responsive — report the dispatch and don't block the main chat on the subagent's result unless the
user explicitly asked to wait.

### Step 1 — Establish the product's use cases

Read the target repo's own identity docs — `README.md`, `AGENTS.md`, `CONTEXT.md` (whichever
exist) — for what the product is actually for and how it's actually used day to day. This
judgment must be derived fresh from the target repo's own docs every time, never assumed or
hardcoded in this skill file. (Same step `/roadmap` performs before sequencing — the underlying
question, "what is this product for," is identical; only what you do with the answer differs.)

### Step 2 — Establish what the product actually does today

Not what the docs or backlog say is planned — the real, currently-implemented surface. Docs can be
aspirational or stale; trust the implementation over the description of it.

For a CLI, this typically means enumerating the actual commands/flags: run the tool's own
`--help` (and subcommand `--help`) output, or skim the command-registration source files (e.g.
`cmd/<tool>/*.go` for a Cobra-style Go CLI) rather than trusting a features doc. This is one
concrete example method, not the only one — for other kinds of projects (a GUI app, a library, a
pipeline/daemon) the dispatched subagent should use its own judgment for the equivalent
ground-truth source (e.g. a library's actual public API surface, a daemon's actual endpoints or
config schema), rather than this skill prescribing a single mechanism that only fits one project
shape.

### Step 3 — Find the gap

Compare Step 1's use cases against Step 2's real surface. For each capability a user pursuing a
stated use case would reasonably expect but that Step 2 didn't turn up, check that it isn't
already tracked — **in both open and closed tickets**, so this never re-proposes something the
project already deliberately rejected or deferred:

- Run `harnez find -d <repo> issues "<candidate topic>" -I` per candidate (fuzzy search covers both
  open and closed tickets by default; inspect the rendered overview PNG via `view_file` for fast visual scanning) before treating anything as a genuine gap.
- For a broader sweep instead of one-candidate-at-a-time, skim `issues/README.md`'s index and the
  closed/archive ticket titles directly for topical overlap.
- Drop (or note as "already covered/already rejected") any candidate that matches an existing
  ticket, open or closed — a closed ticket with a rejection rationale is a strong signal the
  candidate needs a materially different angle to be worth resurfacing at all.

**Be skeptical of your own output here.** LLM-driven gap analysis is prone to proposing generic
"nice to have" feature-itis — capabilities that sound plausible for "a product like this" in the
abstract but don't tie to anything this repo's docs actually said its users need. Before keeping a
candidate, require it to trace to a concrete, *named* use case from Step 1 (quote or paraphrase the
specific sentence/section that motivates it) — not a generic "most CLIs/products have X." If a
candidate only survives on genericism, drop it.

### Step 4 — Report candidates, don't file them

Output a report, not new ticket files:

- One entry per surviving candidate: the gap, the specific named use case it serves, why Step 2's
  survey shows it's genuinely missing (not just under-documented), and confirmation it didn't match
  anything in the open/closed tracker sweep from Step 3.
- Candidates that were considered and dropped for being generic or already covered — brief mention
  is useful so a human reviewer can see the skepticism was applied, not skipped.
- A closing note that filing any of these as real tickets is a separate, human-gated next step —
  once the user picks which candidates are worth tracking, they can go through `/issue`'s
  delegated-filing workflow (`commands/issue.md`). This skill itself never writes to `issues/`.

---

## Non-Goals / Constraints

- **Read-only against the issue tracker.** Never edit any `issues/*.md` ticket file, never touch
  `issues/README.md`, never run `harnez index` or `harnez issues <verb>`. This skill only produces
  a report back to the operator; it writes nothing to the repo.
- Not a replacement for `/roadmap` — sequencing existing tickets stays roadmap's job. Discovery is
  occasional/exploratory; it runs before something becomes a ticket, not instead of sequencing one.
- Not a ticket-filing tool — filing stays human-gated through `/issue`, even when a candidate looks
  obviously worth tracking.
- No fixed scoring rubric for "what counts as a gap." Same philosophy as `/roadmap`'s refusal to
  hardcode a value-judgment formula — the comparison in Step 3 is made per repo by the dispatched
  subagent, informed by that repo's own docs and real surface, not computed from a fixed algorithm.

---

## Final Report

Report back to the operator: the use cases and real surface established in Steps 1–2 (briefly),
the list of surviving gap candidates with their use-case justification, the dropped/covered
candidates and why, and the reminder that filing is a separate `/issue`-driven step the operator
decides on.
