# File a Repository Issue

## TL;DR

Choose the executor from the invocation, not model discretion:

- `/issue <request>` or `/issue direct <request>`: the receiving agent executes the
  filing workflow itself, including duplicate search and drafting. Do not spawn a filer.
- `/issue delegate <request>` or an explicit request to hand filing to an agent:
  delegate the workflow once. Pass any user-selected agent/model/effort unchanged;
  otherwise use the harness default and disclose it. If delegation is unavailable,
  report that limitation instead of silently switching to direct execution.

These are prose selectors, not Harnez CLI flags. A delegated filer reads this skill
completely and follows the direct path; it must not delegate filing again.

The selected executor reads the target repository's applicable instructions and
issue-tracking guidance before acting, then runs the filing workflow end to end:

```bash
harnez find -d <repo> issues "<search terms>" -I  # or inspect rendered PNG via view_file
harnez issues new -d <repo> "<title>"
# Fill the returned ticket path using the repository's issue conventions.
harnez issues open -d <repo> <n> --commit "docs(issues): file <n>, <summary>"
```

On the delegated path, keep the host responsive: report that filing was dispatched and don't block the main chat
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
- The absolute repository and installed skill paths, execution mode (`direct`),
  selected agent/model/effort, write and commit authority, and expected result:
  ticket number/path or duplicate finding, verification, commit, and remaining blockers.

The delegator supplies known context without predoing duplicate search or drafting.
The executor must read the full installed issue skill and applicable repository
instructions, including referenced issue-tracking guidance not already in context.
Do not assume a child inherited these documents or conversation history.

Leave the exploratory steps to the selected executor: duplicate
search, inspecting nearby tickets for local conventions, choosing priority independently
from severity, and drafting the ticket body (problem, scope, acceptance criteria,
verification guidance; for exploratory requests, record uncertainties instead of
inventing implementation details; for multi-step or non-trivial tickets, decompose
Section 3 into numbered milestones M1, M2... with automated verification targets).

On the delegated path, use a compatible, healthy named filer if explicitly selected
or already assigned to this workflow; otherwise create a fresh filer. Freshness is
not a requirement for every ticket. Follow the available harnez-advisor lifecycle
guidance for reuse without changing an advisor's role or permissions. Closed/parked
sessions remain candidates for supported native reuse; closure alone proves no failure.
For Codex usage-limit interruptions, suspend productive dispatch and use bounded
health checks. Quarantine a confirmed dead session, close/retire it using supported
controls, and verify one fresh replacement before dispatch. Keep the exclusion and
replacement mapping outside the repository; quota reset does not clear quarantine.
Inspect any reserved ticket and actual work before replay to avoid duplicate filing.
If native recovery is unsupported or inconclusive, report the blocker; invent no APIs.

Serialize tracker mutations and commits with other writers in the shared repository.
The host owns result collection and targeted child cleanup. Confirm explicit native
compaction before parking a reusable filer; a self-summary is not compaction. Report
unavailable operations honestly. Close disposable filers after collecting their result.

Stage only the new ticket and regenerated index so unrelated working-tree changes remain
untouched. If the repository is not harnez-managed, follow its local tracker and commit
rules instead of assuming `harnez`, `issues/`, or an immediate tracker commit.
