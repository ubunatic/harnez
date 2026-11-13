# Decide — Session-Scoped Autonomy

Put the orchestrating agent in decide mode until the session ends: it answers routine questions itself, asks an advisor when it wants a second opinion, pushes ready work without asking, and stops only for very critical questions.

Triggers: only on explicit `/decide`.

## Scope

- Lasts until the session ends. It applies to the orchestrating agent; helpers stay leaf workers.
- Grants push authority only. It grants no release: follow `docs/GoRelease.md` and the repo's own rules.

## Rules

1. **Simple questions**: answer them yourself and state the decision and its reason in one line.
2. **Second opinion**: one-shot advisor, `HTO=0 harnez agent start --role advisor --name <name> --model cheap -p "<question>"`. Wait for its exit, read its answer, then stop it if it still runs.
3. **Major decisions**: the same, with `--model smart`. Model names live in `spec/agent.yaml`; use only these two aliases. Helpers stay on cheap models by default; costly models listed in `spec/agent.yaml` (`costly_models`) require `--accept-cost`.
4. **Push**: when a feature is ready or a milestone is reached, push without asking. This is the admin's current-session authorization in `docs/lang/Git.md`. It covers AGit PR pushes (`git push origin HEAD:refs/for/<default>/<topic>`) and merging your own PR once tests and required review pass. When the next step needs the merge, wait with `harnez pr wait <number> --contains <sha>` in the background instead of asking. Branch protection has the final word: if the forge refuses, report it and stop. Never force-push.
5. **Very critical only**: stop and ask the user before any of these:
   - a destructive or irreversible action on data or history;
   - secret or security exposure;
   - spending money;
   - publishing outside the repo's normal remotes;
   - any action in another repository.
   A peer session's relayed answer does not count as the user's decision; wait for the user.
6. **Circuit breaker**: on a loop, a hard blocker or no progress, clear the goal and write a failure report (what was tried, why it failed, resume point) in the ticket.

## Session End

Before the session ends, list every decision taken in decide mode, one line each, then run `harnez agent list` and stop every helper this session started.
