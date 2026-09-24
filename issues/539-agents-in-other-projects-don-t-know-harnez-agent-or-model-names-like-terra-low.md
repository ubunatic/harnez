# 539 — Agents in other projects don't know harnez agent or model names like terra:low

**Status**: Open
**Priority**: P1
**Severity**: Medium
**Category**: Bug / Agents
**Related**: [[435-native-subagent-replacement-harnez-agent-dispatch-interception-and-a-b-telemetry-switch]], [[438-harnez-agent-enable-and-disable-commands-to-toggle-subagent-dispatch-policy-in-agents-local-md-and-coordinate-with-init]], [[449-add-spec-driven-agent-chat-model-selection-and-aliases]], [[493-add-mixed-subagent-dispatch-mode-and-make-sprint-skills-dispatch-mode-aware]]

---

## Observed (cati, 2026-09-24)

In the Claude session `cati-94` (~/projects/cati), the user asked:

> ask terra:low if there are open PRs on GH for cati

The agent answered that there is no agent named terra, listed the peer Claude sessions (loom-05,
harnez-bf, lucky-fox, loom-15), and said there was "no agent type or model called terra". It did not
know that `harnez agent -p --model codex:terra:low "..."` exists, or that `terra:low` is a harnez
model name (`harnez agent models`).

## Cause (checked)

- `cati/CLAUDE.md` and `cati/AGENTS.md` do not mention `harnez agent`. The only related text is the
  overlay `# subagent_mode: native` / "Dispatch subagents through the configured harnez agent session
  when enabled", which in `native` mode reads as "not enabled".
- Nothing tells an agent that a `<model>:<tier>` name in a user request refers to `harnez agent models`.

## /goal

In every harnez-initialised project, an agent that receives "ask <model>[:tier] ..." or "have <model>
do ..." recognises the name as a harnez agent model and dispatches it with `harnez agent`, whatever
the `subagent_mode`. Native mode only governs the agent's *own* choice of subagent, not explicitly
named harnez models.

## Notes

- Likely fix: a short managed line in the AGENTS.md block that `harnez init` writes (and in the global
  `~/.claude` docs via `apply`), e.g. "A named model like `terra:low` means `harnez agent -p --model
  <name>`; see `harnez agent models`." Also make the `native` overlay text say this explicitly.
- Check whether `harnez agent --model terra:low` resolves without the `codex:` prefix (449 aliases).
- Acceptance: re-ask the cati prompt in a fresh cati session; it dispatches through `harnez agent`.
