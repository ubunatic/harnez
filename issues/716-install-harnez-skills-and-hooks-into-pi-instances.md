# 716 — Install Harnez skills and hooks into Pi instances

**Status**: Closed — Pi skills and Distill hook managed by apply/status/revert; verified live with Pi 1.0.4 in the 723 smoke test; telemetry hook follows in 725
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [#712 Pi CLI driver](712-add-support-for-the-pi-coding-agent-cli-1-0.md), [#070 cross-agent hook support](070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode.md), [#631 external skills](631-external-skills-canary-then-user-registry-b-then-vendored-skills-a.md)

---

## 1. Problem & Motivation
The Pi driver in #712 lets `harnez agent` launch Pi, but `harnez apply` does
not set up the user's Pi installation the way it sets up Claude, Codex,
Antigravity and Prime Agent. Today only the Distill extension
(`~/.pi/agent/extensions/harnez-distill.ts`) is written for Pi; skills and the
other hooks are not.

**Goal**: Pi becomes one more agent managed by `harnez apply`, `status` and
`revert`, exactly like the existing ones. No Pi-specific lifecycle or command.

## 2. Technical Specification / Findings
- `internal/claude/apply.go:SkillTargetsByAgent` lists Gemini, Codex, Claude
  and Prime skill targets, but not Pi. Add Pi there with a `config.yaml`
  target, following the existing per-agent targets.
- Pi's home is `~/.pi/agent`, or `$PI_CODING_AGENT_DIR` when set (Pi then reads
  everything from there). Skills live at `<pi home>/skills/<name>/SKILL.md`.
  lmcoder's agent image copies Harnez skills to that path and Pi 0.99.2 found
  and used all of them (lmcoder issue 144); confirm on Pi 1.0+.
- Hooks: install the ones Pi's extension API supports, as already done for the
  Distill extension; report any that Pi cannot run.
- Not in scope: another model driver, or listing Pi models in
  `harnez agent models`.

## 3. Implementation & Verification Plan
- Add the Pi skill target and supported hooks to `apply`, `status` and
  `revert`, reusing the code paths of the other agents.
- Tests as for the other agents, plus target resolution from
  `PI_CODING_AGENT_DIR`.
- Live check with the current Pi CLI: ask Pi to list its skills and name the
  one that files an issue; confirm installed hooks run. Update
  `docs/PiAgent.md`.

## 4. Results
- Code (commit ecce40fc): Pi is a fifth skill target via `pi_agent_target`
  (default `~/.pi/agent`, `$PI_CODING_AGENT_DIR` wins); the Distill extension
  follows the same override; `status` prints the Pi home.
- Live check (#723 smoke test, Pi 1.0.4, 2026-10-06): after `harnez apply`,
  Pi's system prompt lists all 26 Harnez skills from `~/.pi/agent/skills`,
  including `issue`; `harnez status` reports them ok; a Pi bash tool call
  fired the Distill extension (`distill hook` in `harnez log`).
- Hooks: Distill is the only Harnez hook Pi has. Codex also gets telemetry
  hooks; the Pi counterpart is #725. The bundled third-party skills are not
  copied to Pi, as for Gemini and Prime (config.yaml `bundled_skills`).
