# 602 — Managed block and architecture doc show 'harnez agent start ... -p', which start rejects

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: [479](479-epic-unified-harnez-agent-cli-name-model-d-p-c-prompt-files.md), [483](483-add-harnez-agent-p-prompt-root-form-with-slash-command-interception.md)

---

## 1. Problem & Motivation
The "Harnez Agent" line in the managed block, which `harnez init` copies into every project's AGENTS.md, reads:
`harnez agent start --detach --name <name> --role <role> --model <model> -p <prompt>`.
`harnez agent start` has no `-p` and fails with `unknown shorthand flag: 'p' in -p`. Only the root form `harnez agent -p` (483) takes `-p`; `start` takes the prompt as a positional argument or through `-f`. Agents copy this line verbatim, so every dispatch that follows the docs fails once. Seen in lmcoder on 2026-09-26, during the issue 119 sprint.

## 2. Technical Specification / Findings
- Template source: `config.yaml:539` (managed "Harnez Agent" block).
- Same mistake in `docs/HarnezAgentArchitecture.md:63` (`harnez agent start --detach --name w --model luna -p "background task"`).
- Downstream copies: every project's AGENTS.md managed block, including harnez's own `AGENTS.md:65`.

## 3. Implementation & Verification Plan
1. Pick one fix: change the docs to the positional form (`... --model <model> "<prompt>"`), or accept `-p/--prompt` on `start` and `resume` as an alias, for consistency with the root form.
2. Update `config.yaml` and `docs/HarnezAgentArchitecture.md`, then run `harnez init` so the harnez `AGENTS.md` is regenerated.
3. Verification: add a test that runs every `harnez agent ...` example command in the managed block through Cobra's flag parsing, so the docs cannot drift from the CLI again.
