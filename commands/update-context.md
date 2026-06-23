Create or update CONTEXT.md in the project root.

# CONTEXT.md
- dense, technical reference for future agentic coding sessions
- not a README or tutorial (no teasers, no long talk)
- provides an overview for agents and developers
- captures things that cannot be derived by reading the code alone
  - why decisions were made
  - what was tried and rejected
  - what is not yet implemented
- document typical gotchas 
  - surpsiring runtime behavior 
  - format requirements by external tools
  - common pitfalls and dead ends

# Structure
- use the following template as an idea
- adapt headings to the project
- keep only what applies
- be creative and radical if needed

The goal is to inform agents and developers and not comply to this template alone.

# Template

## What it is
One paragraph: what the tool/service does and its primary design philosophy.

## Architecture
Source files and their responsibilities. Key data flows. Important invariants.

## Commands / API / Interface
What the user/caller invokes and what each entry point does.

## Generated / managed files
What files are written, by what strategy, and what constraints apply
(e.g. format requirements from external tools).

## Docs and Issues
Where are evergreen, temp., and issue docs located.
Which docs serve what purpose.

## Key design decisions
The non-obvious choices: what alternatives were considered, why this approach
was taken, what constraints forced the decision.

## Gotchas and learnings
Bugs found, format requirements discovered at runtime, failure modes,
anything that would surprise a new contributor.

## Not yet implemented
Planned features that are absent from the current codebase.

## Updating this CONTEXT.md
- Read the full codebase before writing (source files, config, existing docs)
- Prefer concrete facts over vague descriptions. Show formats, not just names
- No padding, no intros, no "this document describes". Start with section facts directly
- Use short phrases and concise sentences
- Use bullets for longer list
- If CONTEXT.md already exists, update it in place — preserve sections that are
  still accurate, rewrite or remove sections that are stale

