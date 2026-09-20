# 450 — Fix hosted agent chat input line after terminal resize

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Reliability

---

## 1. Problem & Motivation

`harnez agent chat` does not reliably preserve the hosted agent's input line
when the terminal is resized. After increasing the terminal height, the hosted
Codex UI can leave the cursor on an output/status line instead of the prompt,
making keyboard input visually misleading or unusable.

Observed output:

```text
Harnez Agent Chat: calm-otter (1e53d609-60ec-4201-8950-d9b1002c993a)

OpenAI Codex (v0.155.1)

• You ran pwd
! # <--- CURSOR is now here, after I increased the height of the terminal

  done 6:29 PM                                                        Shell mode

›
```

## 2. /goal

Keep the hosted agent chat input line anchored and usable after terminal resize
events, including height changes, with the cursor restored to the active prompt
and no corrupted or overwritten output.

## 3. Scope and Constraints

- Reproduce the issue with the existing hosted chat/PTY path and terminal
  resize handling.
- Ensure resize events propagate the current terminal dimensions to the hosted
  session and trigger a safe redraw or prompt repositioning.
- Preserve normal scrollback, status output, shell mode, and interactive input.
- Cover both growing and shrinking terminal dimensions where the PTY/platform
  supports them.
- Do not change provider prompt styling or add provider-specific UI hacks
  unless required by a documented protocol limitation.

## 4. Acceptance Criteria

- Resizing the terminal while `harnez agent chat` is active leaves the input
  prompt at the correct bottom position with the cursor on the prompt.
- Input typed immediately after a resize is delivered to the hosted agent and
  is not inserted into an output/status line.
- Existing chat, attach, control-socket, and normal PTY behavior remains intact.
- Tests or a deterministic PTY-level reproduction cover resize propagation and
  redraw/input-line recovery, including at least one height change.
