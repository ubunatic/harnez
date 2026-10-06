# Pi Agent Driver

Harnez can run the Pi coding-agent CLI (1.0+) as a batch `harnez agent` provider.
The adapter uses Pi's documented JSON mode and persistent session support; install
Pi separately and make the `pi` executable available on `PATH`.

## Models

`--model pi` (or `pi:default`) uses Pi's configured default model. A Pi model can
also be selected directly with `--model pi:<provider/model-id>[:low|med|high]`,
for example:

```sh
harnez agent start --name pi-task --model pi:openai/gpt-4o:high -d . -p "Inspect the tests and report gaps"
```

Model IDs are passed to Pi unchanged. Pi must already have the corresponding
provider/model configured and authenticated. Thinking tiers map to Pi's
`--thinking` option (`med` becomes `medium`). The generic Pi default entry has
no comparable Harnez quota-cost measurement; model-specific cost metadata is not
inferred from Pi's provider catalog.

## Sessions and limitations

Each turn runs Pi in JSON mode with Harnez's selected working directory. Pi's
session header supplies the provider session ID; Harnez uses that ID with Pi's
`--session` option for follow-up turns. Pi persists the transcript in its own
session store, grouped by working directory. Harnez's `--name` labels its own
session record and does not rename Pi's session.

Harnez requests manual compaction through Pi's documented RPC `compact`
command, not by sending `/compact` as a prompt. Pi returns an estimated
post-compaction context size; that is a heuristic, not an exact provider token
count. Pi's own automatic compaction and overflow recovery also remain active.

Pi's CLI invocation is a foreground process. Harnez's process controls bound or
terminate that process; Pi has no separate stop request in this adapter. Deleting
a Harnez session does not delete the corresponding Pi transcript. External Pi
sessions are not imported into Harnez's session list.

JSON events provide per-turn input/output/cache usage when Pi reports it. Pi's
JSON protocol does not provide Harnez with a validated context-window size, so
context-window capacity and reasoning-token counts remain unknown.

## Skills and Hooks Integration

`harnez apply` manages skills and extension hooks for Pi instances, exactly like
it manages Claude, Codex, Antigravity, and Prime Agent.

- **Skills target**: Pi reads skills from `<pi home>/skills/<name>/SKILL.md`.
  `harnez apply` copies Harnez skills to this path. Pi's home defaults to
  `~/.pi/agent` (configured via `pi_agent_target` in `config.yaml`), or
  `$PI_CODING_AGENT_DIR` when set in the environment.
- **Hooks & Extensions**: Harnez installs its Distill extension hook to
  `<pi home>/extensions/harnez-distill.ts`.
- **Status & Revert**: `harnez status` reports installed Pi skills and extension
  hooks; `harnez revert --managed` (or `CleanAll`) cleans up managed Harnez
  artifacts across all Pi targets.
