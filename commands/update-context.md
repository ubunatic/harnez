Create or update CONTEXT.md in the project root.

CONTEXT.md is a dense, technical reference for future Claude sessions — not a
README or tutorial. It captures things that cannot be derived by reading the
code alone: why decisions were made, what was tried and rejected, runtime
gotchas, format requirements imposed by external tools, and what is not yet
implemented.

Structure (adapt headings to the project, keep only what applies):

## What it is
One paragraph: what the tool/service does and its primary design philosophy.

## Architecture
Source files and their responsibilities. Key data flows. Important invariants.

## Commands / API / Interface
What the user/caller invokes and what each entry point does.

## Generated / managed files
What files are written, by what strategy, and what constraints apply
(e.g. format requirements from external tools).

## Key design decisions
The non-obvious choices: what alternatives were considered, why this approach
was taken, what constraints forced the decision.

## Gotchas and learnings
Bugs found, format requirements discovered at runtime, failure modes,
anything that would surprise a new contributor.

## Not yet implemented
Planned features that are absent from the current codebase.

Rules:
- Read the full codebase before writing (source files, config, existing docs).
- Prefer concrete facts over vague descriptions. Show formats, not just names.
- No padding, no intros, no "this document describes". Start each section directly.
- If CONTEXT.md already exists, update it in place — preserve sections that are
  still accurate, rewrite or remove sections that are stale.
