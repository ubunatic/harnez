# 478 — Make listed agent sessions resumable or accurately report terminal state

**Status**: Open

**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: #479

---

## 1. Problem & Motivation

`harnez agent list` presents historical sessions as `completed`, but those
sessions may not be resumable. For example, the listed Codex session
`agent-codex-f7b8e3e8` (`01a0c4af-70a2-7fc2-96b3-daf7ab5824c9`) appears
completed, yet both of these fail at the provider layer:

```text
harnez agent resume agent-codex-f7b8e3e8 "say hi"
Error: ... codex resume: exit status 1
```

The same failure occurs when using the full session ID. This makes it
impossible to tell whether a worker is resumable, permanently terminal, or
corrupted, and blocks recovery workflows such as lean sprints.

## 2. Expected Behavior

- A session shown as resumable must successfully accept `agent resume` with
  its displayed ID or name.
- If a completed session cannot be resumed by design, `agent list` must expose
  that terminal/non-resumable state clearly and `agent resume` must return an
  actionable error explaining why.
- Session IDs and names displayed by `agent list` must resolve consistently in
  `agent resume`.
- Add diagnostics that preserve the underlying provider error without reducing
  it to an unexplained exit status.

Investigate provider/session persistence, model configuration, and the
difference between a worker's completed state and a resumable conversation.

## 3. Verification

- Add regression coverage for list → resume using both full ID and displayed
  name.
- Cover successful resume, genuinely terminal sessions, provider failure, and
  missing/invalid identifiers.
- Verify the user-facing status and error messages distinguish each case.

/goal

Make agent lifecycle state truthful and recoverable: every listed resumable
session can actually be resumed, while non-resumable sessions are explicitly
identified with actionable diagnostics.

## Epic note (#479)

Prerequisite for #482: session attribution must only select sessions that are actually resumable, so a reliable resumable/terminal state is needed first.
