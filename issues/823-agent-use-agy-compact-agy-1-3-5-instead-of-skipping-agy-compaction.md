# 823 — agent: use agy /compact (agy 1.3.5+) instead of skipping agy compaction

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 644 (agy resume skips harnez compaction), 646 (jev compaction)

---

## 1. Problem

agy 1.3.5 added an on-demand `/compact` command. `/compact` on its own summarizes the
conversation and ends the turn. `/compact <message>` summarizes and then answers the
message. agy 1.4.2 is installed.

harnez still assumes agy cannot compact. `cmd/harnez/agent_run.go` (around line 761) says
"agy auto-compacts and has no /compact command" and leaves agy out of
`EnsureContextUnderThreshold` (644). `AgyDriver.Compact` (`internal/subagent/agy.go`)
already sends `/compact` with a resume call, but nothing calls it for agy sessions.

644 also found that agy's usage counts add up every model call in a turn. That makes a
threshold check on those numbers misfire, so the threshold needs a sound context
measure before it can trigger compaction.

## 2. Open questions (canary first)

- Does `agy -p --conversation <id> "/compact"` run the command in print mode, or is it
  only interactive? `--disable-slash-commands` suggests print mode expands it. Does the
  output report how much smaller the context became?
- Does the interactive control path (`SendControl ... "compact"`) reach agy's `/compact`?
- Which number gives agy's current context size for the threshold, and where does it come
  from? If none is reliable, add an explicit `harnez agent compact` for agy and leave the
  automatic threshold out.

## 3. Goal

/goal harnez agent can compact agy sessions with agy's `/compact`: on demand, and
automatically when a reliable context size exists. Verify with a live canary on a real
agy session and with driver tests, then update the 644 comment and docs. Stop and report
if the canary shows `/compact` does not work non-interactively, or if a quota or
permission limit blocks the canary.
