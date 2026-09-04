# File a Repository Issue

## TL;DR

Don't search for duplicates, dig back through the conversation or repo, or draft ticket
content yourself. Compose a self-contained handoff prompt (see below) and dispatch a fresh
subagent to run the filing workflow end to end:

```bash
harnez find -d <repo> issues "<search terms>"
harnez issues new -d <repo> "<title>"
# Fill the returned ticket path using the repository's issue conventions.
harnez issues open -d <repo> <n> --commit "docs(issues): file <n>, <summary>"
```

Keep the host responsive: report that filing was dispatched and don't block the main chat
on the subagent's result, unless the user explicitly asked to wait or the very next step in
the conversation truly cannot proceed without the ticket number.

Keep this workflow deliberately thin: the repository instructions and issue-tracking
documentation should already be in context. Inspect and follow them rather than
restating their full policy here.

## Handoff Prompt

The subagent has no memory of this conversation — brief it like a colleague who just
walked in, never "as discussed above." At minimum, include:

- The concrete ask, close to verbatim.
- Any motivating context or reasoning already discussed that explains *why* this is
  being filed.
- Any files, commits, or other tickets already touched or mentioned this session that
  are relevant to the new ticket.
- Any explicit constraints the user stated (scope limits, priority hints, what *not*
  to do).

Leave the exploratory steps to the subagent — that's the point of delegating: duplicate
search, inspecting nearby tickets for local conventions, choosing priority independently
from severity, and drafting the ticket body (problem, scope, acceptance criteria,
verification guidance; for exploratory requests, record uncertainties instead of
inventing implementation details).

Stage only the new ticket and regenerated index so unrelated working-tree changes remain
untouched. If the repository is not harnez-managed, follow its local tracker and commit
rules instead of assuming `harnez`, `issues/`, or an immediate tracker commit.
