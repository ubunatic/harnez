# 450 — Fix hosted agent chat input line after terminal resize

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Reliability

---

## 1. Problem & Motivation

`harnez agent chat` does not reliably preserve the hosted agent's input line
when the terminal is resized. After increasing the terminal height, hosted
Codex and Claude UIs can leave the cursor on an output/status line or garble
the display, making keyboard input visually misleading or unusable.

Observed output:

```text
Harnez Agent Chat: calm-otter (1e53d609-60ec-4201-8950-d9b1002c993a)

OpenAI Codex (v0.155.1)

• You ran pwd
! # <--- CURSOR is now here, after I increased the height of the terminal

  done 6:29 PM                                                        Shell mode

›
```

Claude reproduction:

```text
Harnez Agent Chat: calm-tiger (8b3766ff-df7d-4cc9-9fff-b89a5cff2654)
 ▐▛███▛█   Claude Code v2.1.278
✻▜Crunched fork5s ·5d·nel18.31
  🤖…▝▝    ~/projects/harne5      200

❯  # <-- CURSOR moved up after height increase
! 🤖…                              75
  I'm/inmthewharne
```

## 2. /goal

Keep the hosted agent chat input line anchored and usable after terminal resize
events, including height changes, with the cursor restored to the active prompt
and no corrupted or overwritten output. Treat this as a hosting/TUI problem for
`../loom`: implement the reusable terminal hosting surface there first, then
make `harnez agent chat` a Loom app rather than accumulating provider-specific
alternate-screen and redraw logic in Harnez.

## 3. Scope and Constraints

- Reproduce the issue with the existing hosted chat/PTY path and terminal
  resize handling for both Codex and Claude.
- Investigate and implement the hosting primitive in `../loom`, including a
  safe alternate-screen/pane strategy and resize propagation.
- Integrate `harnez agent chat` as a Loom app using that hosting primitive.
- Ensure resize events propagate the current terminal dimensions to the hosted
  session and trigger a safe redraw or prompt repositioning.
- Preserve normal scrollback, status output, shell mode, and interactive input.
- Cover both growing and shrinking terminal dimensions where the PTY/platform
  supports them.
- Do not change provider prompt styling or add provider-specific UI hacks
  unless required by a documented protocol limitation. Avoid handcrafting a
  second alternate-screen manager in Harnez when the reusable Loom host can
  own it.

## 4. Acceptance Criteria

- Resizing the terminal while `harnez agent chat` is active leaves the input
  prompt at the correct bottom position with the cursor on the prompt.
- Input typed immediately after a resize is delivered to the hosted agent and
  is not inserted into an output/status line.
- Both Codex and Claude remain readable and interactive after resize; no
  provider UI is garbled or displaced.
- The reusable hosting behavior lives in `../loom`, and Harnez's chat command
  uses Loom as its TUI host rather than duplicating terminal-screen management.
- Existing chat, attach, control-socket, and normal PTY behavior remains intact.
- Tests or a deterministic PTY-level reproduction cover resize propagation and
  redraw/input-line recovery, including at least one height change.
