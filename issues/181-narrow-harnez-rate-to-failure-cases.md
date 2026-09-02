# 181 — Narrow `harnez rate` to Failure/Unexpected-Outcome Cases

**Status**: Closed
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [issues/142](142-disable-rate-feedback-and-measure-overhead.md) (disabling rate feedback and measuring overhead)

---

## 1. Problem & Motivation

The current Tool Feedback Protocol asks agents to call `harnez rate <tool> <1-5> "<summary>"
"<project>/<ticket>"` after *every* internal tool call. In practice this is both chatty (adds a
tool call + narration overhead after nearly every action) and ignored — across observed sessions
agents routinely skip it entirely despite the instruction being present in global CLAUDE.md. A
policy that is both high-friction and not followed is worse than no policy: it adds instruction
weight without producing signal.

## 2. Technical Specification / Findings

Change the policy (and, if `harnez rate` has any built-in prompting/reminder logic, the tool
behavior) so that rating is expected only when:

- A tool call failed (non-zero exit, error result, exception).
- A tool call succeeded but did not produce the expected outcome (e.g. a fix that didn't fix,
  a search that missed the target, a build that passed but the feature still didn't work).

Routine successful calls (a `Read` that returned the right file, a `Grep` that found the match)
should NOT be rated — this is the inverse of today's "rate everything" framing and should reduce
call volume by roughly the fraction of calls that go as expected (typically the large majority).

This is primarily a documentation/instruction change (global `CLAUDE.md`'s "Tool Feedback
Protocol" section, and any per-project mirrors), but check whether `harnez rate` itself has
argument validation, help text, or a reminder mechanism that assumes "rate every call" and would
need updating to avoid contradicting the new narrower policy.

## 3. Implementation & Verification Plan

1. Grep the harnez repo and global `~/.claude` docs for every place the current "rate after every
   tool call" instruction is stated (global CLAUDE.md, `docs/` bundled copies, any `harnez rate`
   help/usage text).
2. Rewrite the instruction to the failure/unexpected-outcome-only framing; keep the exact command
   syntax and scoring rubric (5 flawless / 3 partial / 1 useless) since those aren't the problem.
3. Check `internal/claude/toolfeedback_test.go` (recently touched per git status) for any test
   assumptions tied to the old "rate everything" framing that need updating.
4. Verify with `go test ./...`; update `issues/README.md`.

## 4. Resolution Note

Both source-of-truth locations for the instruction were config.yaml-driven, not hand-authored
copies, so there was exactly one place per delivery mechanism to change:

- `config.yaml`'s `agents_md.global.sections` entry named "Tool Feedback Protocol" (renders into
  `~/.claude/CLAUDE.md` and its `~/AGENTS.md`/`~/.prime/agent/AGENTS.md` mirrors via `harnez
  apply`'s managed-section mechanism).
- `config.yaml`'s `skills` entry named `tool-feedback-protocol` (renders into
  `~/.claude/skills/tool-feedback-protocol/SKILL.md`), including its `description:` line which
  previously said "Use immediately after completing any internal tool call" — that description
  is itself an instruction surface (skill-picker text), so it was rewritten to state the
  failure/unexpected-outcome condition rather than "any call".

Both were rewritten to: rate only when a call (a) failed, or (b) succeeded but missed the
expected outcome — not after routine successful calls. The exact `harnez rate <tool_name> <1-5>
"<summary>" "<project>/<ticket>"` syntax and the 5/3/1 scoring rubric were kept verbatim per the
ticket's constraint. The worked example was changed from a success case ("found the bug") to a
failure case ("missed target, wrong file") so the example itself models the new policy instead of
contradicting it.

`internal/claude/toolfeedback_test.go` needed no changes: its assertions only check for the
literal `harnez rate <tool_name> <1-5>` substring and a rough 15-60 word budget on the section
content, neither of which encodes the old "rate everything" framing in a way that would make the
test wrong under the new policy. The new section content lands at ~57 words, still inside that
budget.

`harnez rate` itself (cmd/harnez rate subcommand) has no argument validation, help text, or
reminder logic that assumes "rate every call" — it just accepts an already-decided score and
writes a telemetry record, so no code changes were needed there. (Issue 183's proactive-reminder
work will need to consult this narrowed policy when it decides when a "gap" exists — noted as
context, not implemented here.)

Verification: `go build ./...` and `go test ./...` pass (all packages). `make install` and
`make apply` were both run; `make apply` confirmed the rewritten section text and skill file are
now live under `~/.claude/CLAUDE.md` and `~/.claude/skills/tool-feedback-protocol/SKILL.md`.

No scope was trimmed from the original ticket text.
