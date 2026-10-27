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

### 3b. Track every compaction through Pre/PostCompact hooks (owner direction, 2026-10-27)

Owner: every supported agent should have pre- and post-compact hooks, so harnez can record each compaction (whether
one ran, why, and what it changed) by session ID, and use that record for all compaction decisions.

- Live state: harnez installs no `PreCompact`/`PostCompact` hook for any agent (no match in `internal/`, `cmd/`,
  `spec/`, `~/.claude/settings.json`). Telemetry already stores compaction events and economics
  (`internal/telemetry/insert.go` `InsertCompactionEvent`, `InsertCompactionEconomics`), fed from stream parsing
  today, not hooks.
- Hook support per agent: Codex has both (owner transcript above; `docs/AutoCompaction.md`). Claude Code has
  `PreCompact`; whether 2.1.311 has `PostCompact` must be re-checked (180 found none at the time). agy and Pi:
  unknown, check docs and record. Where an agent lacks a hook, fall back to the stream event (Claude
  `compact_boundary`, Codex `context compacted`) and say so in the record.
- **agy finding (owner-supplied research, 2026-10-27, agy 1.4.8, read from the hook docs embedded in the binary):**
  no `PreCompact`/`PostCompact`. The six hook events are `SessionStart`, `PreToolUse`, `PostToolUse`,
  `PreInvocation` (before each model call), `PostInvocation` (after tool calls finish) and `Stop`; none fires at
  compaction. Config: `~/.gemini/config/hooks.json`; each hook gets JSON on stdin with `conversationId`,
  `transcriptPath`, `modelName`. The binary has compaction internals (`OnCompactionArgs`, `CompactionInfo`, a
  "Compacting conversation..." label) but no user-facing hook name for them.
  - Substitute (untested inference): a `PostInvocation` hook reads `transcriptPath`, logs step count and any
    compaction marker; consecutive values reveal a compaction and give before/after. First step for agy: run one
    `/compact` and see how the marker appears in the transcript file before building on it.
- The hooks write one event per compaction: session ID, agent, model, trigger (native limit, harnez threshold,
  on-demand `agent compact`, CLI's own auto), context before/after when the payload carries it, timestamp. Pre
  without a matching Post = failed or interrupted compaction.
- Consumers: `agent compact` confirms the drop from the Post event instead of guessing; 985's cost check and 969's
  re-compact and post-compact fill guards read the same records; `harnez agent list` / stats can show compaction
  count per session.
- Installed through `harnez apply` like the other hooks; hook commands and payload fields per agent in `spec/`.

## 4. Other options

- Fresh session per milestone instead of compaction (lean-sprint policy, or a `resume --fresh` command; see 985):
  no summary cost, no stale context; loses the worker's in-session knowledge, which the ticket channel mostly
  replaces.
- Lower Codex's native limit only (no on-demand path): one spec change, but no host control and no confirmation.
- Let 985's check recommend compact vs. fresh by measured economics (`telemetry` already stores compaction
  economics, `internal/telemetry/economics.go`).

## 5. Verification

- The owner's commands recorded as measured per provider in `docs/AutoCompaction.md`.
- Hook support per agent recorded there (has Pre / has Post / stream fallback).
- Live: one compaction per agent (claude, codex, agy) leaves a Pre and a Post record with the same session ID,
  readable by harnez; a killed compaction leaves a Pre without Post and is reported as such.
- Tests: Codex compact args (low limit, status prompt, `-C <dir>`), acknowledgement required before the next prompt,
  per-model threshold lookup, agy on-demand path.
- Live: `harnez agent compact --name <codex session>` prints the before/after context and the status line; a
  resumed Codex session past its model threshold compacts before the prompt.
- `make test-q1`; `make install`.
