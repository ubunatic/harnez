# Write a Study

Write a concise study of a sprint, retrospective, or topic in `docs/studies/`.

## Output Location & Naming

- Path: `docs/studies/<YYYY-MM-DD>-<kebab-case-title>.md`.
- Start with YAML front matter containing the kind. Use `---` / `kind: <kind>` / `---` so later
  stories can find their predecessor; keep existing keys such as `title` and `weight`.
- Kinds: `story` (overall project progress and agentic-flow story), `sprint` (per-ticket or per-epic
  report), `retro` (transcript retrospective), `canary`, `benchmark`, and `topic` (single-concern
  study). Use the kind that describes the study; overall `kind: story` reports are written by the
  `story` skill.
- `.harnez/repo.yaml` is the switch for indexed studies. When enabled, create `docs/studies/` if
  needed and link the document in `docs/README.md` (or `docs/index.md`).
- If `.harnez/repo.yaml` sets `docs.studies: false`, follow `.harnez/rules/Index.md` for the
  local-docs/session fallback.

## Required Sections

Every study must include:

1. **Context** — scope, starting state, and intended goals.
2. **Outcome** — what was delivered or learned, with relevant evidence.
3. **What Went Wrong** — failures, false assumptions, or near-misses; how each was caught and the
   exact fix. Say when there were none.
4. **Learnings** — concrete, actionable lessons.
5. **Follow-Up Tickets** — review failures, near-misses, open questions, and recommendations. File
   actionable follow-ups with `harnez issues new` in the owning repository, link the study from each
   ticket, and list allocated ticket numbers here. Record justified non-goals.

## Ticket Extraction Gate

Before presenting the study, complete the follow-up ticket review above. Ask each allocated worker:
"What ticket number was allocated, and have all actionable follow-ups from this study been filed and listed here?"

## Workflow

1. Gather evidence appropriate to the study and distinguish observations from inferences.
2. Write the study with the required sections and links to related work.
3. Complete the **Ticket Extraction Gate** before presenting the study.
4. Index it only when the repository enables indexed studies.
