# To PRD

Turn the current conversation into a PRD and publish it to `issues/` — no interview, just
synthesis of what has already been discussed. Do NOT interview the user; just synthesize
what you already know.

## Process

1. Explore the repo to understand the current state of the codebase, if you haven't already.
   Use the project's domain glossary vocabulary throughout the PRD (see `/domain-modeling`),
   and respect any ADRs in the area being touched.

2. Sketch out the seams at which the feature will be tested. Prefer existing seams to new
   ones, and the highest seam possible — the fewer seams across the codebase, the better; one
   is ideal. If new seams are needed, propose them at the highest point possible. Check with
   the user that these seams match their expectations before moving on.

3. Write the PRD using the template below, then publish it as a new file under `issues/`
   (e.g. `issues/<slug>.md`), following this repo's existing issue convention: a status field
   showing what's done vs. open, and cross-references to relevant Evergreen docs (`docs/`).
   Update `issues/README.md` or `issues/index.md` if either exists, the same way `/evergreen`
   does when it creates new issues.

<prd-template>

## Problem Statement

The problem that the user is facing, from the user's perspective.

## Solution

The solution to the problem, from the user's perspective.

## User Stories

A LONG, numbered list of user stories. Each user story should be in the format of:

1. As an \<actor\>, I want a \<feature\>, so that \<benefit\>

<user-story-example>
1. As a mobile bank customer, I want to see balance on my accounts, so that I can make better informed decisions about my spending
</user-story-example>

This list of user stories should be extremely extensive and cover all aspects of the feature.

## Implementation Decisions

A list of implementation decisions that were made. This can include:

- The modules that will be built/modified
- The interfaces of those modules that will be modified
- Technical clarifications from the developer
- Architectural decisions
- Schema changes
- API contracts
- Specific interactions

Do NOT include specific file paths or code snippets. They may end up being outdated very
quickly.

Exception: if a prototype produced a snippet that encodes a decision more precisely than
prose can (state machine, reducer, schema, type shape), inline it within the relevant
decision and note briefly that it came from a prototype. Trim to the decision-rich parts —
not a working demo, just the important bits.

## Testing Decisions

A list of testing decisions that were made. Include:

- A description of what makes a good test (only test external behavior, not implementation
  details)
- Which modules will be tested
- Prior art for the tests (i.e. similar types of tests in the codebase)

## Out of Scope

A description of the things that are out of scope for this PRD.

## Further Notes

Any further notes about the feature.

## Status

Open — not yet started.

</prd-template>
