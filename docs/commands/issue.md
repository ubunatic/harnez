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

If `.harnez/repo.yaml` sets `issues.tracker: none`, follow `.harnez/rules/Index.md` for the local-tracker/session fallback.

```bash
harnez find -d <repo> issues -a status:active
harnez find -d <repo> issues "<search terms>"
harnez find -d <repo> issues similar "<search terms>"
git -C <repo> fetch          # reserved numbers must see other clones' tickets
harnez issues new -d <repo> "<title>"
# Fill the returned ticket path using the repository's issue conventions.
harnez issues open -d <repo> <n> --commit "docs(issues): file <n>, <summary>"
```

Use text for routine issue discovery. Add `-I/--image` only when the user explicitly
requests image output; an assumed token benefit is not a reason to render PNG cards.
The active index `issues/README.md` covers Open, In Progress, Blocked, and Draft tickets;
it links to the generated Backlog and Closed indexes.

## Publish Promptly

The goal is a clean ticket commit that reaches review as soon as it is filed. Publishing to `main` is the exception, not the default.

1. Commit on a topic branch. Under `git.mode: team` with `protected_main: true` in `.harnez/repo.yaml`, `harnez` refuses commits on the default branch. If you are on it, create `issue/<n>-<slug>` first.
2. Take the first route that the repository policy and the remote permissions allow:
   - **Direct push to `main`**: only if `protected_main` is `false`, or the repository administrator explicitly authorizes it in the current session, and the remote accepts the push. A machine or user name alone is not authorization.
   - **Contributor AGit PR**: otherwise push with `git push origin HEAD:refs/for/main -o topic=<topic>`, where `<topic>` is the contributor's reusable topic (typically the Git username). Reuse the open PR for that topic: later ticket commits pushed to the same topic update it. Do not open one PR per ticket. If that PR was already closed or merged, use a new topic and say so. Never force-push.
   - **Stop**: if the remote refuses AGit or the user lacks permission, leave the commit on the topic branch and report the denial. Do not push to `main` as a workaround.
3. Report the state exactly: "committed on <branch>", "pushed to <remote> <ref>", or "AGit PR <number or URL> opened or updated". An AGit change reaches `main` only after the PR is merged. Say "published" or "on main" only after the remote confirms it.

Number collisions are secondary to publishing, but never allow a duplicate or an overwrite. If the remote already holds a different ticket with the reserved number, do not overwrite or rename it: fetch, allocate a fresh number with `harnez issues new`, and delete only your own unpublished placeholder after checking it. If that cannot be resolved cleanly, stop and report the collision.

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
  selected agent/model/effort, write, commit and publication authority (direct push, AGit PR or stop),
  and expected result: ticket number/path or duplicate finding, verification, commit, publication
  status, and remaining blockers.

The delegator supplies known context without predoing duplicate search or drafting.
The executor must read the full installed issue skill and applicable repository
instructions, including referenced issue-tracking guidance not already in context.
Do not assume a child inherited these documents or conversation history.

Leave the exploratory steps to the selected executor: duplicate
search, inspecting nearby tickets for local conventions, choosing priority independently
from severity, and drafting the ticket body:
- **Always define a `/goal` with an exit clause**: Every issue must explicitly define a `/goal` (or clear Goal statement) that specifies the desired end-state, definition of done, and when to stop if blocked on user input or a denied permission. Example: `/goal Implement the fix and verify it, or stop and report when blocked on a user decision or denied permission.` The exit clause includes the circuit breaker: on a loop, a hard architectural or OS blocker, or no progress, clear the goal and write a failure report saying why it failed.
- **Keep issues short and brief**: When the goal is clear, keep tickets lean without over-specifying low-level implementation details or hypothetical steps. Only add numbered Milestones (`M1`, `M2`...) when the task genuinely requires staged execution, distinct review gates, or multi-phase handoffs.
- **Issue Handling & Live Status Verification**: Because issues may sit in the backlog for days or weeks as the project evolves, any agent picking up a ticket must always check the live project/code status and recent commit history before starting work, re-verifying assumptions against current code rather than blindly following stale references. The ticket's `/goal` serves as the durable north star.
- For exploratory requests, record uncertainties instead of inventing implementation details.

On the delegated path, use a compatible, healthy named filer if explicitly selected
or already assigned to this workflow; otherwise create a fresh filer. Freshness is
not a requirement for every ticket. Load harnez-advisor guidance only when the user
explicitly requests advisor reuse; ordinary delegation needs no advisor skill.
Reuse must preserve the session's role and permissions. Closed/parked
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
untouched. In team/protected mode (`git.mode` in `.harnez/repo.yaml`), the ticket commit rides
on the current topic branch; on the default branch, create `issue/<n>-<slug>` first. If the repository is not harnez-managed, follow its local tracker and commit
rules instead of assuming `harnez`, `issues/`, or an immediate tracker commit.
