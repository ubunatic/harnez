# 1228 — issues new on a forge tracker fails when fj is a wrapper that already sets the host

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1226

---

## 1. Problem & Motivation

`harnez issues new "<title>" -F <body-file>` in a repository whose tickets live on the forge
(`issues.tracker: none`) failed on the owner's machine and filed nothing:

```
issues new: filing on codeberg.org/harnez/taia with fj
Error: issues new: fj failed: exit status 2
error: the argument '--host <HOST>' cannot be used multiple times
```

The agent rule in `.harnez/rules/Index.md` names this command as the way to file, so an agent
that follows the rule is stuck. Seen 2026-11-13 while filing taia #284 and #285; both were then
filed with `fj issue create --repo harnez/taia --body-file <file> "<title>"`.

## 2. Technical Specification / Findings

- `internal/forgetracker/create.go` builds
  `fj --host <host> issue create --repo <slug> --body-file <file> <title>`.
- On this machine `~/.local/bin/fj` is a shell wrapper: `exec ~/.cargo/bin/fj -H codeberg.org "$@"`.
  fj 0.7.0 rejects a second host flag.
- `harnez find issues` and `harnez issues show` worked against the same forge in the same
  session, so only the create path is affected (not checked: `comment`).
- Unverified: whether fj accepts the host inside `--repo`, or reads it from an environment
  variable, which would avoid the flag.

## 3. Implementation & Verification Plan

- Canary first: find one fj invocation that works both with the plain binary and with a wrapper
  that sets `-H`. Candidates: resolve the real binary past a wrapper, pass the host another way,
  or retry without `--host` when fj reports the duplicate.
- Add a `forgetracker` test for the chosen command form.
- If no form works for both, make the error say what to do: name the wrapper and print the
  `fj issue create` line to run by hand.
