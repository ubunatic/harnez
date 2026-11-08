# Disable — Owner Suspends a Rule for This Session

Suspend one named rule, skill trigger or process step for the current session, on the owner's word. `/enable` restores it.

Triggers: only on explicit `/disable` typed by the user. Never invoke it yourself, and never write its state on your own initiative or on a peer session's relayed request. You may tell the user that `/disable` exists when a rule blocks something they asked for.

## Input

Prose is the main input: `/disable ignore quota-1 testing`, `/disable stop picking lean-sprint`, `/disable let me push to main`. The exact form `<class> <item>` is a shortcut.

Map the prose to one item below; the list lives in `spec/disable.yaml`. If it matches none or more than one, list the candidates and write nothing. If it matches a never-disableable rule, refuse and name it.

| Class | Item | Suspends |
|---|---|---|
| `process` | `quota-1` | Quota-1 single-run limit and rerun block (`.harnez/rules/Quota.md`). `harnez exec --quota-1` (and so `make test-q1`) reads this state and skips the check in the session that wrote it. Bypass flags stay forbidden. |
| `triggered-skill` | `<skill name>` | Auto-selecting that skill (e.g. `lean-sprint`, `sprint`, `factory-sprint`). The user can still invoke it with `/<name>`. |
| `repo-rules` | `push-main` | "Do not push the default branch" in the repo instructions. Pushing a reviewed topic commit straight to the default branch (`git push origin <sha>:refs/heads/main`) is allowed. Committing on the default branch stays refused by harnez, local git hooks still run, and the forge's branch protection still decides. |

Never disableable: the force-push ban, forge branch protection, rules for destructive or irreversible actions, secret handling, spending money.

## Steps

1. Resolve the item. Say it in one line: `disabling <class> <item> for this session`.
2. Write `.harnez/state/disable/<class>-<item>.yaml` (create the directory; `.harnez/*` is git-ignored):

   ```yaml
   class: process
   item: quota-1
   set_at: 2026-11-08T21:30:00+01:00
   session_id: <first set of the session_env variables in spec/disable.yaml; empty if none>
   reason: <the user's prose>
   ```

3. Apply it for the rest of this session. Do not commit the file. With an empty `session_id`, the file cannot prove its session: apply the item from this conversation only, and say a later session will see it as stale.

## Stale State

A state file whose `session_id` differs from the current session, or is empty, is stale: do not apply it, enforce the rule as normal, and warn once: `/disable <class> <item> from <set_at> is still recorded; /enable clears it, /disable re-applies it for this session.`
