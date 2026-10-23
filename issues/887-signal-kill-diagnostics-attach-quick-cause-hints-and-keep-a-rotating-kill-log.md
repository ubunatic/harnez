# 887 — Signal-kill diagnostics: attach quick cause hints and keep a rotating kill log

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 256 (rolling collector logs), 255 (collector diagnostics), 543 (OOM kills), 885 (swap backstop)

---

/goal When a provider fetch or other child process is killed by a signal, harnez records what it can
learn about the cause within a few seconds (OOM kill, provider rejection, own timeout, other sender) next
to the kill message, and appends every kill to a rotating log on disk; verified with a test per detected
cause; or stop and report when a cause cannot be detected safely or a user decision is needed.

## Problem

On 2026-10-23 22:34:37 the usage UI showed `Claude/GPT`, `Gemini 4 Argon` and `Gemini` as
`unavailable (signal: killed)`. The collector journal only said
`agy -p "/usage" … elapsed 15.724s vs timeout 2m0s, cause: external signal (signal: killed)`
(`internal/usage/registry.go:271`). It wasn't our timeout and the kernel log showed no OOM, but who sent
the signal stayed unknown. The next fetch at 22:39 succeeded.

## Requirements

- On a signal exit, run bounded checks (a total budget of a few seconds, no scanning of the whole system), for
  example: the kernel/journal OOM record for the PID and time, the cgroup `memory.events` oom_kill counter,
  whether harnez's own timeout or cancel fired, the child's last stderr lines (a provider rejection such as an
  auth or quota error from Google), and the exit signal number.
- Attach the findings to the kill message (journal and UI hint), and mark each one as found or unknown,
  never as a guess.
- Append every kill (time, command, PID, elapsed, signal, findings) to a rotating log, for example under
  `~/.local/state/harnez/`, with size/count limits in `spec/` (docs/Spec.md).
- Unknown remains a valid result: a check that fails or times out must not delay or break the fetch loop.

## Open questions

- Which checks work without root on the user's systems (journalctl `-k` access, cgroup v2 paths)?
- Should 256's rolling log host this, or should it be a separate kill log?
