# 718 — Show the resolved model tier in the agent session header

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: `cmd/harnez/agent_stream.go` (`turnStream.info`), `subagent.Session.Tier`

---

## 1. Goal

/goal The `[session info: …]` header of `harnez agent start|resume` shows the tier (reasoning
effort) the session actually runs with, so a host can confirm a requested tier such as `:high`
without reading the session store; or stop and report when a provider gives no way to know it.

## 2. Problem

On 2026-10-05 a host started `--model codex:gpt-6-luna:high`; the header printed
`agent=codex:gpt-6-luna` only, so the host had to tell the owner it could not confirm "high".
The tier is stored (`Tier: m.Tier` in `agent_run.go`, `agent_async.go`), it is just not printed:
`info()` formats `agent=%s` from provider and model.

## 3. Verification

- Header shows e.g. `agent=codex:gpt-6-luna:high` for an explicit tier and the default tier when
  none was given (marked as default); `resume` shows the stored tier.
- Unit test on the header line for start with and without a tier, and for resume.
