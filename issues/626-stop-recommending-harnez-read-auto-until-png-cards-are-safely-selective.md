# 626 — Stop recommending harnez read --auto until PNG cards are safely selective

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Agent Efficiency / Reliability
**Related**: #395, #419, #440, #543, #546

---

/goal No harnez-generated guidance, hook or subagent prompt leads an agent to receive a PNG card
unless it passed `-I` explicitly; `--auto` never picks images until the user re-enables it for
cases Harnez can safely detect; stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation

User observation (2026-09-28): in a codex session the agent called `harnez read` and got a PNG card
back, with the old "Reading" prefix that was never fixed. The user wants PNG card reading only
very selectively, and `--auto` image selection off indefinitely until a case with a clear, safely
detectable benefit is decided.

## 2. Technical Specification / Findings

Host grep on HEAD found three places that steer agents to `--auto` (which may choose images):
- `cmd/harnez/hook.go` `readRedirect`: the read-redirect hook suggests `harnez read --auto`.
- `internal/subagent/subagent.go` `readGuidance`: subagent prompts say `harnez read --auto <file>`
  and "open returned PNG paths with an image tool".
- `internal/bench/tasks.yaml`: bench prompts use `--auto{{card}}` (bench-only, may stay).

To do:
- Make `--auto` resolve to text for every provider (or remove its image branch behind a spec
  switch, per `docs/Spec.md`), and change the hook and subagent guidance to plain
  `harnez read` / `-n -L`.
- Find the "Reading" prefix (not found by a plain grep for `"Reading `; may be drawn into the PNG
  or come from a card header, see #440) and either fix or record it.
- Check installed global docs/rules (`~/.claude`, `~/.codex`, agy) for `--auto`/`-I` advice.
- Relation to #546 (per-project `-I` recommendation toggle): this ticket is the global default-off.

Finding: `internal/readcard` renderers do not add a `Reading` prefix. `harnez read`
text mode prints the selected source content directly; explicit image mode prints
`See @<png>` and the PNG card title is the source filename. The guard text's
“Reading & Context Discipline” phrase is its policy name, not a read-output prefix.

## 3. Implementation & Verification Plan

- Tests: `--auto` returns text for claude/codex/gemini profiles; hook and subagent guidance
  contain no `--auto`.
- `make install`, `harnez apply`; a codex subagent read of a >100-line file returns text.
