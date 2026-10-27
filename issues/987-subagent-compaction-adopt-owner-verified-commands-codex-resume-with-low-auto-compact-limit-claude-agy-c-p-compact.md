# 987 — subagent compaction: adopt owner-verified commands (codex resume with low auto-compact limit, claude/agy -c -p /compact)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 985 (per-turn cost check in agent output), 969 (per-model compaction limit, draft), 869 (agy print-mode compaction skipped), 644 (agy context unknown), 590 / 594 (codex resume compaction), 673 (claude compaction check), `docs/AutoCompaction.md`

---

/goal Every provider harnez drives can be compacted on demand and at a per-model threshold, using the commands the
owner verified, and `harnez agent compact` confirms the context dropped; or stop and report when blocked on an owner
decision or a CLI that does not behave as observed. Circuit breaker: on a loop or no progress, clear the goal and
write a failure report here.

## 1. Problem & Motivation

Sprint 983 (2026-10-27) kept one Codex developer (`dev-983`, `codex:gpt-6.1-terra:med`) for 18 turns. Cached
re-read reached 47M tokens and the session hit the Codex usage limit before its last commit (details in 985).
Nothing compacted it: harnez cannot compact a Codex session on demand, and Codex's own trigger sat at the global
200k default.

Owner-verified commands (**measured** by the owner, 2026-10-27; installed versions that day: codex-cli 0.164.0,
Claude Code 2.1.311, agy 1.4.8). No re-test needed; record them in `docs/AutoCompaction.md` as measured:

- `agy -c -p "/compact"` and `claude -c -p "/compact"` compact well.
- `codex exec -c model_auto_compact_token_limit=50000 -C <dir> resume <id> "summarise your known status in one line"`
  forces a compaction (slash commands cannot be sent to `codex exec`) and returns a cheap one-line status as a bonus.

Owner transcript (codex-cli 0.164.0, `gpt-6.1-terra` medium, project `neus`, human-readable output, not `--json`):

```
codex exec -c model_auto_compact_token_limit=50000 -C ~/projects/neus resume 01a17c6c-... "summarise your known status in one line"
user
summarise your known status in one line
hook: PreCompact
hook: PreCompact Completed
context compacted
hook: PostCompact
hook: PostCompact Completed
hook: SessionStart              (x4 started, x4 completed)
codex
Issue 102 M3: changes committed and tests and install passed; pinned replay still misses some evidence, ...
hook: Stop
hook: Stop Completed
tokens used
107.088
```

What it shows:

- Compaction runs before the prompt is answered, in the same `exec` call; the reply is a usable one-line status.
- The marker is the line `context compacted`, between the `PreCompact` and `PostCompact` hooks. The matching
  `--json` event is still to be read from one real run (harnez drives Codex with `--json`).
- That call reported 107,088 tokens used (the "107.088" uses a dot as the thousands separator): the compaction
  turn costs about one full read of the old context.
- `SessionStart` hooks fire four times after the compaction. If harnez's own SessionStart hook re-injects its
  preface each time, the fresh context starts fuller than needed; check during implementation (link 969's
  post-compact fill guard).

## 2. Current state (live code, 2026-10-27)

| Provider | `auto_compact` mode (`spec/agent.yaml:16`) | On-demand `Compact()` | Threshold |
|---|---|---|---|
| claude | manual | `Resume(id, "/compact")` (`internal/subagent/claude.go:85`), drop verified via `compact_boundary` (673) | 200k global, Haiku 80k |
| codex | native | returns "manual /compact is unsupported" (`internal/subagent/codex.go:271`) | `-c model_auto_compact_token_limit=<threshold>` on every run and resume (`codex.go:125,144`), 200k for all Codex models |
| agy | disabled | `Resume(id, "/compact")` (`internal/subagent/agy.go:164`), never triggered automatically | none; agy context count unreliable (644, 869) |

Gaps:

1. Codex has no on-demand compaction, so `harnez agent compact` and the pre-prompt trigger cannot shrink a Codex
   session; only Codex's own limit applies.
2. No per-model Codex threshold: Terra, Sol and Luna all compact at 200k.
3. agy compaction works on demand but nothing triggers it, because harnez has no reliable context count for agy.
4. The Claude path uses `--resume <id>`; the owner's form is `-c` (continue the latest session in the dir). Check
   whether they behave the same in print mode; `--resume <id>` is safer when several sessions share a dir.

## 3. Proposal (careful; other options in §4)

- **No canary for the commands** (owner-measured, §1). The only open check is the shape of Codex's compaction event
  in `--json` output, read from one real run during implementation, so the acknowledgement test uses real values.
- **Codex `Compact()`:** a resume with `-c model_auto_compact_token_limit=<low>` (value in `spec/`) and the one-line
  status prompt. Confirm via Codex's compaction event (594 has the acknowledgement parsing); surface the status line
  to the host as the compaction result. Keep the replay safety from 590 (prompt sent only after the compaction is
  confirmed).
- **Per-model Codex thresholds** in `spec/agent.yaml` `compact_thresholds`, the same mechanism Haiku uses.
- **agy:** enable on-demand compaction through the host (`harnez agent compact`) and from 985's cost check, which
  works without a context count (turn count, cached re-read); keep automatic triggering off until 644/869 give a
  count.

## 4. Other options

- Fresh session per milestone instead of compaction (lean-sprint policy, or a `resume --fresh` command; see 985):
  no summary cost, no stale context; loses the worker's in-session knowledge, which the ticket channel mostly
  replaces.
- Lower Codex's native limit only (no on-demand path): one spec change, but no host control and no confirmation.
- Let 985's check recommend compact vs. fresh by measured economics (`telemetry` already stores compaction
  economics, `internal/telemetry/economics.go`).

## 5. Verification

- The owner's commands recorded as measured per provider in `docs/AutoCompaction.md`.
- Tests: Codex compact args (low limit, status prompt, `-C <dir>`), acknowledgement required before the next prompt,
  per-model threshold lookup, agy on-demand path.
- Live: `harnez agent compact --name <codex session>` prints the before/after context and the status line; a
  resumed Codex session past its model threshold compacts before the prompt.
- `make test-q1`; `make install`.
