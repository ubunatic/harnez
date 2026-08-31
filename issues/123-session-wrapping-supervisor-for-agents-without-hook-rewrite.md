# 123 — `harnez run <agent>`: session-wrapping supervisor for agents without hook-rewrite support

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Architecture
**Related**: [[118-harnez-exec-shell-interceptor]], [[119-harnez-hook-agent-hook-management]], `docs/HookRewritePattern.md`, `docs/studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md`

## Decision (2026-08-31)

115–122 target the **independent-tool** invocation model: harnez is a peer
CLI the agent's own hook/tool-use machinery calls out to per event
(`harnez rate`, `harnez exec hook`, `apply`-installed hooks). Harnez
never launches or owns the agent process. This matches the existing
shape of every other harnez feature (`distill`, `usage`) and is the
right default wherever a native hook-rewrite point exists — proven for
Claude Code, unverified for AGY, and per the 2026-08-19 study, likely
*absent* for Codex ("does not provide a generic hooks.json lifecycle
dispatch").

For an agent with no usable hook-rewrite point, the independent-tool
model has no way to intercept shell-tool calls at all — there's nothing
to reach out to it from. Discussed as a deliberate second track: a
**session-wrapping supervisor**, `harnez run <agent>`, that spawns the
target agent (`claude`, `agy`, `codex`) as a child process, owns its
stdin/stdout(/PTY), and intercepts commands at the process-execution
layer instead of via agent-native hooks. This sidesteps the
hook-capability gap entirely but is a materially larger build (PTY
handling, output passthrough fidelity, signal forwarding, and likely
its own command-detection heuristics rather than clean tool-call JSON).

**Scope of this ticket**: this is filed as a placeholder for the second
track, explicitly out of scope until:

1. [[119]]'s per-agent canary work confirms which of AGY/Codex actually
   lack a usable hook-rewrite point (don't build this against an
   assumption — verify the gap first).
2. The independent-tool model (115–122) has shipped for at least Claude
   Code, so there's a working reference to compare fidelity/overhead
   against.

## Why not build this first / for everyone

- Session-wrapping is strictly more invasive: it must faithfully proxy
  an interactive PTY session (see `docs/studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md`
  for how much this repo has already learned the hard way about PTY/
  terminal-rendering fidelity bugs) rather than handling one well-formed
  JSON payload per tool call.
- It doesn't get native `tool_name`/`tool_input` structure — command
  detection would have to be inferred from raw terminal output or shell
  history, a strictly harder and less reliable parsing problem than the
  hook JSON contract.
- Every agent that *does* support hook rewrite (Claude, likely AGY) gets
  a strictly better result from the independent-tool model — no reason
  to pay the supervisor's overhead/fidelity cost there.

## Acceptance Criteria (future — not yet actionable)

- [ ] [[119]]'s canary work has confirmed at least one target agent
      (expected: Codex) has no usable hook-rewrite point.
- [ ] A canary (per `docs/other/Canary.md`) demonstrates `harnez run
      codex -- <args>` can transparently proxy an interactive Codex
      session with no observable behavior change when telemetry capture
      is off, before any capture logic is added.
- [ ] Command-detection approach for the wrapped session (parsing shell
      invocations from the child's actual exec() calls, e.g. via
      ptrace/strace-style interception, vs. heuristic output scraping)
      is decided and canary-verified — this is the hardest open design
      question and should not be assumed away.

## Notes

Not scheduled — this ticket exists so the "session-wrapping" idea
raised during the 2026-08-31 architecture discussion isn't lost, and so
115–122 aren't blocked re-litigating it. Revisit once [[119]]'s
Antigravity/Codex canaries land.
