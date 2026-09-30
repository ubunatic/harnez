# 656 — harnez agent send: opt-in messaging to detached agents, after host session agent tracking

**Status**: Open
**Priority**: P2 (Medium) — start only after 630 (start/resume/wait backgrounding guides) is closed
**Severity**: Moderate
**Category**: Agentic Ergonomics / Feature
**Related**: 630 (session vs machine background), 611 (host polling), 439 (agent chat lifecycle), 477 (hook-driven completion), 483 (root agent form, slash interception)

---

## 1. Problem & Motivation

Hosts can start agents but cannot talk to an agent that runs fully detached, and a host
does not reliably know which agents it has started. 630 fixes how hosts run agents in the
session background. This ticket designs a real eventing layer on top: `harnez agent send`.

## 2. Scope, in order (each milestone gates the next)

- **M1 Host session agent tracking.** Record, per host session, the number and names of all
  agents it started (start, resume, detached). Keep the host continuously informed of what was
  started (e.g. hook-injected context or the `start`/`resume` output), so it never loses track.
- **M2 Statusline count.** Show the host session's detached agent count in each agent's
  statusline (Claude, AGY, Codex where a statusline exists).
- **M3 Delivery discovery (canary, per agent).** For Claude Code, Codex and AGY, find what
  **actually** works to deliver a message into a running or idle agent, not what docs claim:
  - piggyback on the agent's native messaging (e.g. Claude `SendMessage`, AGY/Codex equivalents);
  - hooks (inject the message as context on the next prompt/tool event);
  - just resume with another prompt (`harnez agent resume --name X "<msg>"`);
  - anything else found. Record the recommended way and the proven ways per agent in
    `docs/studies/`, with probe evidence (`docs/Canary.md`).
- **M4 `harnez agent send --name <name> -m <msg>`.** Implement with the M3 winners per agent,
  only after M1-M2 work. Delivery must be confirmed or reported as failed, never silent.

## 3. Constraints

- Opt-in. Either a user-callable skill (e.g. `/harnez-peering`) or a `harnez apply` toggle;
  M3 decides which fits better. Default off.
- No new external daemon unless M3 proves nothing native works; the agents stay "the system"
  (see 630).
- Re-check live code and 630's outcome before starting.

/goal Hosts always know which agents they started, statuslines show the detached count, and
`harnez agent send` delivers a message to a named detached agent through a per-agent method
proven by canary; stop and report when blocked on a user decision (opt-in form) or a denied
permission.
