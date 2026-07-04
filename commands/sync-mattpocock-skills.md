# Sync mattpocock/skills

Check whether the upstream sources for the commands we adapted from
[mattpocock/skills](https://github.com/mattpocock/skills) have changed, and report a summary
so the user can decide what to port over. Do not edit any files in this step unless the user
asks you to after seeing the diff.

## Source files this integration is based on

- https://raw.githubusercontent.com/mattpocock/skills/main/skills/engineering/domain-modeling/SKILL.md
- https://raw.githubusercontent.com/mattpocock/skills/main/skills/engineering/domain-modeling/CONTEXT-FORMAT.md
- https://raw.githubusercontent.com/mattpocock/skills/main/skills/engineering/domain-modeling/ADR-FORMAT.md
- https://raw.githubusercontent.com/mattpocock/skills/main/skills/productivity/grilling/SKILL.md
- https://raw.githubusercontent.com/mattpocock/skills/main/skills/engineering/grill-with-docs/SKILL.md
- https://raw.githubusercontent.com/mattpocock/skills/main/skills/engineering/to-prd/SKILL.md

These map to our commands as: the first three → `commands/domain-modeling.md`, `grilling`
→ `commands/grilling.md`, `grill-with-docs` → `commands/grill-with-docs.md`, `to-prd` →
`commands/make-spec.md`.

## Process

1. Fetch each URL above.
2. Compare against our adapted command file. Ignore differences that are expected and
   permanent — our versions target `issues/` instead of an issue tracker (`to-prd`), don't
   reference `setup-matt-pocock-skills`, and don't reference `to-issues` or any other
   upstream skill we haven't pulled in. Do not propose re-adding those references.
3. Summarize only substantive upstream changes: new guidance, changed rules, bug fixes to the
   process, template changes. Present as a short list, grouped by our command file.
4. Ask the user which changes (if any) to port over. Apply only what they approve.

## Explicitly out of scope

Do not suggest pulling in other skills from the upstream repo (`triage`, `to-issues`,
`codebase-design`, `setup-matt-pocock-skills`, `tdd`, `diagnosing-bugs`, `research`,
`handoff`, `teach`, the misc guardrail skills, etc.) as part of this sync — that's a separate
decision for the user to raise explicitly, not something to bundle into a drift report.
