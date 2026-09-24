# 528 — issues open/verbs: warn when a ticket's heading number differs from its file number

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Tooling
**Related**: [[525-replace-recent-skill-rules-with-harnez-runtime-feedback]], [[527-agy-live-quota-fetch-is-killed-on-most-agent-turn-boundaries]]

## Problem

On 2026-09-24 the host wrote a ticket body headed "# 526" into the file
`issues new` returned as 527, then ran `harnez issues open 526`, which acted on
another writer's ticket 526. harnez accepted both silently.

## /goal

`harnez issues <verb>` and `harnez index` warn (stderr) when a ticket's `# NNN`
heading doesn't match its file number, naming both — runtime feedback, not a rule.
