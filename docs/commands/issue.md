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

### Forge tracker

Harnez reads issues on the origin forge. A private repository may need a read
forge token. Search matches terms in titles and bodies within the configured scan
budget:

```bash
harnez find -d <repo> issues "search terms"
```

A recognised forge origin is the tracker when `issues/` is absent, or when
`issues.tracker: none` and `issues/` has no numbered ticket files (`<digits>-*.md`)
or `archive/` directory. For a forge tracker, no `--forge` flag is needed:

```bash
harnez issues new -d <repo> "Title" -F body.md --label bug --dry-run
harnez issues new -d <repo> "Title" -F body.md --label bug
harnez issues comment -d <repo> 42 -F note.md --dry-run
harnez issues comment -d <repo> 42 -F note.md
harnez issues show -d <repo> 42 --json
harnez issues edit -d <repo> 42 -F body.md --if-updated <updated_at>
harnez issues close -d <repo> 42 "done"
harnez issues reopen -d <repo> 42
```

With a local tracker, target the forge explicitly:

```bash
harnez issues new -d <repo> --forge "Title" -F body.md --label bug
harnez issues comment -d <repo> --forge 42 -F note.md
harnez issues show -d <repo> --forge 42 --json
harnez issues edit -d <repo> --forge 42 -F body.md --if-updated <updated_at>
harnez issues close -d <repo> --forge 42 "done"
harnez issues reopen -d <repo> --forge 42
```

Forgejo writes use a separate write token, created with `harnez forge token create
--write`; the read token remains separate. Without a write token, `new` falls back
to fj or tea and `comment` to fj. GitHub `new` uses gh. Labels must exist on the
forge: the API resolves their names to IDs; fj cannot set labels.

**Inbox fallback (never debug tokens)**: If forge issue creation fails for any
reason — missing repository write access (`401`/`403`), missing or unauthenticated
token/CLI (`fj`, `tea`, `gh`), or running inside a worker session — `harnez issues
new` automatically writes the issue to `issues/inbox/<slug>.md` and warns not to
troubleshoot token setup. Agents must **never** spend turns inspecting token
stores, environment variables, or forge CLI auth when filing fails: verify or
write `issues/inbox/<slug>.md`, commit it, and push it on a branch or AGit PR.

`--dry-run` previews forge writes, but does not cover the `new --remote` or
`CLAUDE_CODE_REMOTE` flow, which can create an issue. Forge writes are host-only:
worker markers guard against accidents, not security attacks. Native subagents
inheriting the host environment, workers that unset the markers, and direct forge
tools are not stopped (for `issues new`, workers fall back to `issues/inbox/`).
GitHub writes beyond the existing `new` path are tracked in ticket 1209. The edit
timestamp check is a preflight: a write between the check and PATCH is not detected.
A comment body must contain non-whitespace text and cannot use stdin. Add `--closed`
to comment on or edit a closed issue.

- When an existing ticket partly covers the request, say so and ask before filing or commenting. Comment on other tickets only with the user's OK.
- Report "filed as #N at <URL>; no commit" (or, on inbox fallback, report the `issues/inbox/<slug>.md` path and publish the branch/PR per Publish Promptly). Publish Promptly below applies to file-based tickets and `issues/inbox/` fallbacks.

```bash
harnez find -d <repo> issues -a status:active
harnez find -d <repo> issues "<search terms>"
harnez find -d <repo> issues similar "<search terms>"
git -C <repo> fetch          # see tickets already pushed by other clones (unpushed reservations stay invisible)
harnez issues new -d <repo> "<title>"
# Fill the returned ticket path using the repository's issue conventions.
harnez issues open -d <repo> <n> --commit "docs(issues): file <n>, <summary>"
```

Use text for routine issue discovery. Add `-I/--image` only when the user explicitly
requests image output; an assumed token benefit is not a reason to render PNG cards.
The active index `issues/README.md` covers Open, In Progress, Blocked, and Draft tickets;
it links to the generated Backlog and Closed indexes.

## Publish Promptly

The goal is that the ticket commit reaches `main`, or a review PR, as soon as it is filed.

Routes, in order:

1. **Direct push to `main`**, preferred whenever the admin's authorization allows it and the remote accepts it. Standing consent counts only where applicable policy allows it. Where applicable policy requires authorization in the current session, standing consent is insufficient: ask for it, or use the AGit route. Several policy sources may apply, such as `AGENTS.md`, `.harnez/repo.yaml` or `docs/Git.md`. The strictest rule wins, and the remote's branch protection has the final word.
2. **AGit PR to `main`**, when direct push is not allowed and the remote supports AGit. Use the contributor's reusable topic (by default the Git username). Several issues may share one topic and one PR.
3. **Stop** otherwise. Leave the commit on the current branch, or on `issue/<n>-<slug>` if you are on the default branch, and report the reason.

`<sha>` is the intended issue commit: the commit that `harnez issues open --commit` made for this ticket.

On protected main, an authorized ticket commit can use `harnez issues open --push-main`, or a matching-session `/disable repo-rules push-main`. These unlock issue commits only; follow the applicable authorization and publishing rules above.

Direct route:

- `git -C <repo> fetch origin main`
- Only the ticket change may go out. `git -C <repo> log --oneline origin/main..<sha>` must list only this ticket's commits, and `git -C <repo> diff --stat origin/main <sha>` must touch only `issues/` files. If any unrelated commit or file is included, stop and report it. Do not push.
- `git -C <repo> push --dry-run origin <sha>:refs/heads/main`, then `git -C <repo> push origin <sha>:refs/heads/main`. Never use `--force` or a `+` refspec. A rejection means `main` moved: fetch, re-check, and report.
- After the push, fetch again and check `git -C <repo> merge-base --is-ancestor <sha> origin/main`. Equality is not required, because `main` may advance after the push.

AGit route:

- `git -C <repo> push origin HEAD:refs/for/main/<topic>`. This is the form the repository owner reports the Forgejo remote accepts; it is not verified on other servers. If the remote rejects it, stop and report the server's message. Do not fall back to a direct push.
- Pushes to the same topic update its open PR. Check the topic's PR is still open before pushing. If it was closed or merged, use a new topic and say so.
- The topic may contain other issues' commits. Check that the commits being pushed contain no unrelated changes. If any pushed commit is unrelated to an issue, stop and report it.
- The PR is not on `main` until it is merged. Report it as open, not as published.
- To learn the merge without asking the owner, start `harnez pr wait <number> --contains <sha>` as a background command (on Claude Code, Bash `run_in_background`). It polls Codeberg or GitHub, and its exit code is the only merge signal: 0 merged and `<sha>` is on the default branch; 2 closed without a merge; 3 still open at the timeout; 4 API error; 5 merged but `<sha>` is not on the default branch. Its exit wakes the agent. Do not poll it by hand, and do not report "merged" from chat alone.

Number collisions: `harnez issues new` skips numbers that open pull requests on the forge already use (it warns and goes on if the forge is unreachable; `--offline` skips that check). Closed and merged pull requests do not count. Before publishing, run `git -C <repo> fetch origin main`, then `harnez issues lint --against origin/main`. If it names a collision, run `harnez issues new` for a fresh number, move the draft into the new path, and update its number. Keep the old file until the move is confirmed with `git -C <repo> status`. Never delete or overwrite content. Stop and report the paths only if this cannot be done cleanly.

Report one state per ticket: "on `main` at <sha>, confirmed as an ancestor of `origin/main`"; "AGit PR <number or URL> open or updated, not on `main` until merged"; or "committed locally on <branch>, not published: <reason>". Say "published" or "on main" only after the remote confirms it.

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
