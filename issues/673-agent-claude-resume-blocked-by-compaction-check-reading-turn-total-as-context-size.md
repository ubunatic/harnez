# 673 — agent: claude resume blocked by compaction check reading turn total as context size

**Status**: Closed — Fixed: claude context size from last call/compact_boundary not turn total (1da8887f), zero-usage synthetic events skipped and compact ack read (033f0f95, 650a5022), Compact no longer sends --model empty which made every claude /compact fail with a 400 reported as success (783a2aa2); live: dev-656-fix (stored 441k) resumed twice, verified /compact to 20.9k then direct; study docs/studies/2026-10-01-claude-resume-compaction.md
**Priority**: P1 (High) — breaks the lean-sprint resume loop for every Claude developer
**Severity**: Major
**Category**: Bug
**Related**: [[594-agent-codex-compaction-never-acknowledged-over-limit-resume-always-fails]], [[644-agent-agy-compaction-never-acknowledged-resume-always-fails]], [[656-harnez-agent-send-opt-in-messaging-to-detached-agents-after-host-session-agent-tracking]]

---

## 1. Problem & Motivation

`harnez agent resume` of a Claude worker after one long turn always fails:

```
Error: refusing to send resume prompt: compaction completed without a verified context drop (before 441684, after 0 full input tokens)
```

Seen twice in the 656 sprint on 2026-10-01 (`dev-656-sonnet`, before 849070; `dev-656-fix`,
before 441684), both `claude:sonnet` with exactly one turn. The work was lost to the loop: a new
developer had to be started to finish each step.

## 2. Technical Specification / Findings

- `harnez agent status --name dev-656-fix` shows `Tokens: 449682 (turn 449682)`. That's the
  cumulative input over every tool step of one turn, not the context size. Sonnet's context can't
  hold 449k, so the "over threshold, compact first" decision is wrong. The same class as 644 (agy)
  and 594 (codex).
- After `/compact`, `VerifyCompaction` (`internal/subagent/compact.go`) gets `ContextTokens == 0`
  for Claude and refuses. Either Claude's compact turn reports no usage, or the parser drops it.

### Canary (2026-10-01, `claude -p --model haiku --output-format stream-json --verbose`, 3 Bash steps)

| Source | input + cache_read + cache_creation |
|---|---|
| call 1 | 16334 |
| call 2 | 16548 |
| call 3 | 16707 |
| call 4 (last) | 16854 |
| `result.usage` (turn total, what harnez used) | 66443 (4 calls summed) |

`--output-format json` has only the turn total, so the driver now uses stream-json with `--verbose`
and takes the last main-thread assistant call (sub-agent calls with `parent_tool_use_id` are skipped).
`/compact` turn: `result.usage` is all zeros (this is the `after 0`), but a
`system/compact_boundary` event carries `compact_metadata.pre_tokens=16885`, `post_tokens=2092`;
that post value is the verified size after compaction. With neither, context is unknown (-1).

## 4. Host Review of 1da8887f (2026-10-01): live check failed, one fix left

Live: 4 resumes (dev-673 twice, dev-656-fix twice) all still failed with
`compaction completed without a verified context drop (before 346607 / 441684, after 0 ...)`.

Probe (host ran harnez's exact command, `claude -r <id> --model sonnet -p --output-format stream-json --verbose "/compact"`
on dev-673): the events are `compact_boundary` with `pre_tokens=44835, post_tokens=4033`, then **two
`assistant` events whose usage is all zeros**, then a `result` with zero usage. The parser sets `last` to
the zero-usage assistant event after the boundary, so ContextTokens = 0.

That also confirms the bug: the real context was 44,835, while harnez had stored 346,607 (the turn total).

**Pre-Work / Required Refinements:**
- Skip assistant events whose usage sums to 0 (input + cache_read + cache_creation == 0); they're synthetic.
- Add a test from this real event sequence (boundary, then 2 zero-usage assistant events, then a zero result)
  that expects ContextTokens == post_tokens.
- Sessions stored by the old parser still hold the turn total. Their first resume compacts once, and that
  must now verify via post_tokens. The host checks this on dev-656-fix.

## 3. Implementation & Verification Plan

- Canary first: resume a Claude session with a long multi-step turn, record the raw usage of the
  last API call vs. the turn total, and of the `/compact` turn.
- Use the last call's input (input + cache read + cache creation) as the context size for Claude;
  mark it unknown (negative, which skips compaction) when it isn't available, as 644 did for agy.
- Verify: resume a Claude worker twice after a 400k+ cumulative turn; both resumes are delivered.

## 5. Host Live Check of 650a5022 (2026-10-01): one more root cause

- **Passed:** `dev-673b` resumed after a 224k turn total with no compaction (context now from last call).
- **Failed:** `dev-656-fix` (stored 441684 from the old parser) still refused, now `after -1`.
- The parser is right: fed the real `/compact` stream (`--model sonnet`), it gives ContextTokens 4745 and
  VerifyCompaction passes (host probe test, not committed).
- **Root cause:** `ClaudeDriver.Compact` calls `Resume(ctx, id, "/compact", Model{})`, which runs
  `claude ... --model "" ...`. With harnez's exact args, Claude returns `result` subtype `success` with
  text `Error during compaction: API Error: 400 model: String should have at least 1 character`, and no
  compact_boundary. No harnez compaction of a Claude session has ever worked.

**Pre-Work / Required Refinements:**
- Omit `--model` (and `--effort`) when the model name/tier is empty, or pass the session's model to Compact.
- Treat a result text starting with `Error during compaction` as an error that carries that text, even
  though subtype is `success`.
- Test: Compact's args contain no empty `--model`; the error result maps to an error.
