# 746 — harnez compose <what>: print composed snippets, first claude-channels-prompt

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics / Feature
**Related**: 743 (plugin channel route), 740 (channel), 656 (agent send)

---

/goal Add `harnez compose <what>` printing a ready-to-use snippet, with `claude-channels-prompt` as the first `<what>`, so the user's shell alias can embed it instead of copying a long quoted prompt; unit-tested and installed, or stop and report when blocked on a user decision.

## 1. Problem & Motivation
The always-on channel alias (743) needs `--channels plugin:harnez-channel@harnez-local` plus a long
`--append-system-prompt` trust text. Today the user copies that text from `harnez channel setup`
into the alias; any wording change in harnez leaves the alias stale.

## 2. Specification
- `harnez compose <what>` prints one composed snippet to stdout, nothing else (no banner), so
  `$(harnez compose claude-channels-prompt)` works in an alias or script.
- First `<what>`: `claude-channels-prompt` — the trust/reply startup prompt currently printed by
  `harnez channel setup`. Single source: both commands read it from `spec/channel.yaml`, no Go copy.
- Unknown `<what>` fails and lists the available names; `harnez compose` alone lists them.
- Second `<what>` (user decision 2026-10-07): `claude-channels-args` prints the full launch args,
  `--channels plugin:harnez-channel@harnez-local --append-system-prompt <prompt>`, shell-quoted so
  `eval "command claude $(harnez compose claude-channels-args) \"\$@\""` keeps the prompt as one
  argument (plain `$(...)` would word-split it). Test the quoting with a real `sh -c` round trip.
- Still check that `compose` collides with no existing or planned command name.

## 3. Verification
- Unit tests: exact output equals the spec text; unknown name lists candidates; setup and compose
  print the same prompt.
- `make install`; user checks `claude --channels plugin:harnez-channel@harnez-local --append-system-prompt "$(harnez compose claude-channels-prompt)"` in their alias.
