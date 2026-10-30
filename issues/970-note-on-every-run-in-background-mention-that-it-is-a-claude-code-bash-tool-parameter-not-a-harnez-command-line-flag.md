# 970 — Note on every run_in_background mention that it is a Claude Code Bash tool parameter, not a harnez command-line flag

**Status**: Closed — Note added to every run_in_background mention (Go strings, tests, config, docs) by forgewerk job qar35we3, reviewed ready by claude:opus:med, merged as ubunatic/harnez#9 (c5078c31)
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Docs
**Related**: 630 (host-native background jobs, closed), 666 (HTO=0 in background instructions, closed), 581 (dispatch discoverable), 969 (research session where the flag was wrongly passed)

---

/goal Every agent-facing dispatch instruction, and every long-doc mention that names a harnez command, carries the note below, and tests pass; or stop and report when blocked on a denied permission or a failing test that needs a decision.

## 1. Problem & Motivation

`run_in_background` is a parameter of Claude Code's Bash tool, not of `harnez`. On 2026-10-26 an agent passed it as
`harnez agent resume ... --run-in-background`, got `unknown flag: --run-in-background`, and repeated it once more after
reading the error. No doc defines it as a harnez flag; the agent borrowed the Bash tool's parameter name because the
instructions name it right next to `harnez` commands without saying which tool owns it.

## 2. Technical Specification / Findings

**Note wording** (one line, same everywhere, after the mention or in the same sentence):

> `run_in_background`: Claude Code Bash tool parameter only; not a harnez flag.

**Where it goes, ranked** (grep of `run_in_background` on 2026-10-26):

1. **Dispatch instructions agents read when they start or wait on a worker** — all get the note:
   - `internal/agentpolicy/policy.go:38` (rendered into `.harnez/rules/Local.md:28`; regenerate, don't hand-edit)
   - `internal/mcp/server.go:64` (`sessionBackgroundInstruction`)
   - `cmd/harnez/agent_async.go:83` (help text) and `:126` (printed wait hint)
   - `config.yaml` source of `.harnez/rules/Tools.md:63` (the "Start:" line)
   - `docs/practices/AgenticLoop.lite.md:69` (root copy `docs/AgenticLoop.md:69` follows by sync)
2. **Long docs, only where the mention names a harnez command** — get the note:
   - `docs/practices/AgenticLoop.md:35` (`HTO=0 harnez agent start`)
   - `docs/commands/lean-sprint.md:58`, `docs/commands/lean-sprint-v1.md:50` (`harnez agent start`)
   - `docs/OrchestratedAgentFlow.md:120` (`harnez agent resume … &`)
3. **No note:** `docs/lang/Bash.md:237`, `docs/practices/AgenticLoop.md:454`, `docs/CodexSettings.md:72`,
   `docs/HookRewritePattern.md:113`, `docs/HarnezAgentArchitecture.md:403` (no harnez command in the mention);
   `internal/exec/exec.go:1245` (reads the Claude hook's own `tool_input` field).
4. **Tickets:** closed tickets stay unedited. Open tickets (for example 870) are not edited by this ticket.

**Tests:** `internal/agentpolicy/policy_test.go:91`, `internal/mcp/server_test.go:86` and `:218`,
`cmd/harnez/agent_wait_test.go:485` and `:528` assert the substring `run_in_background: true`, which the note keeps.
`cmd/harnez/testdata/help.golden:311` and `:403` hold the `agent_async.go:83` help text and must be regenerated.

## 3. Implementation & Verification Plan

1. Add the note to each rank 1 and rank 2 place. Check: `rg -n 'not a harnez flag' internal cmd config.yaml docs`
   lists all of them.
2. In each test above, add an assertion for `not a harnez flag` next to the existing one; regenerate `help.golden`.
   Check: the tests fail without the note and pass with it.
3. Regenerate the rendered copies: `harnez init -d .` for `.harnez/rules/` and `docs/AgenticLoop.md`; `git checkout --`
   any unrelated file it rewrites. Check: `TestRootDocCopiesMatchSources` passes.
4. Check: `make test` passes; `make install` runs.
