# harnez agent start -d does not set the agent working directory

Status: Open

## Problem
`harnez agent start --name respect-harnez --model sol:med -d /home/uwe/projects/harnez -f task.md`
ran the codex (sol) agent in the caller's cwd `/home/uwe/projects` (a non-repo
parent of several repos). The agent asked "Which repository should I audit?"
and did no work. The help text says `-d` is used for session attribution, but
users naturally read `-d <dir>` as the work directory (other harnez commands
use `-d` as the repo/project dir).

The flash38 (agy) agent dispatched the same way found loom on its own, so
the behaviour differs between providers.

## Expected
`-d <dir>` also sets the subagent's process working directory for all
providers (claude, codex, agy), or a separate flag exists and `-d` warns when
it differs from cwd.

## Acceptance
- Agents started with `-d X` run with cwd X across providers.
- Test asserting the provider command is spawned with `Dir = X`.
- Help text states the behaviour.
