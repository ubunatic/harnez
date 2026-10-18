# 823 — agent: use agy /compact (agy 1.3.5+) instead of skipping agy compaction

**Status**: Blocked — Developer leaf-role guard rejects live harnez agent compact; host must verify saved flash38:low canary before closure
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

## 4. Canary findings (2026-10-18, agy 1.4.2)

Rechecked current code before editing: `AgyDriver.Compact` already resumes with
`/compact`, and `compactSession` already calls it for batch agy sessions. The
claim above that nothing calls it is stale. Only automatic threshold compaction
is skipped, correctly, because context size remains unknown (644).

Scratch cwd: `/tmp/harnez-823-32z1ebu_`. Exact commands (no tool-using tasks):

```sh
agy --version
# 1.4.2
agy --add-dir /tmp/harnez-823-32z1ebu_ --model gemini-3.8-flash --effort low --output-format json -p 'Remember canary marker amber-823. Reply only OK. Do not use tools.'
# exit 0; stderr empty
# {"conversation_id":"dcafdf24-0c84-446e-9afc-a38f20ccb56a","status":"SUCCESS","response":"OK\n","duration_seconds":50.046498124,"num_turns":1,"usage":{"input_tokens":11794,"output_tokens":1,"thinking_tokens":0,"cache_read_tokens":0,"total_tokens":11795}}
agy --conversation dcafdf24-0c84-446e-9afc-a38f20ccb56a --add-dir /tmp/harnez-823-32z1ebu_ --model gemini-3.8-flash --effort low --output-format json -p /compact
# exit 0
# {"conversation_id":"dcafdf24-0c84-446e-9afc-a38f20ccb56a","status":"SUCCESS","response":"","duration_seconds":62.537126957,"num_turns":2,"usage":{"input_tokens":11794,"output_tokens":1,"thinking_tokens":0,"cache_read_tokens":0,"total_tokens":11795}}
# stderr: warning: run ended with no output and no recorded error
```

Observed side effect, checked using Python sqlite3 read-only connections and
SQLite backup snapshots of `~/.gemini/antigravity-cli/conversations/<id>.db`:
`steps` grew from 2 to 5 rows; a new completed `step_type=23` contained
`<summary>` with `Core request: Remember canary marker amber-823` and the
retained constraints. Thus print-mode `/compact` really summarizes, despite
its empty JSON response. It does not report a context reduction. The unchanged
11794 input / 11795 total are usage totals, not a post-compaction context size.
**Observed context reduction: unavailable, not measured**; database growth is
not a context measurement. JSON has no current-context field; the local database
stores opaque protobuf metadata with no supported context-size column. No
reliable current-context number was established, so keep automatic threshold
compaction disabled and `ContextTokens=-1`.

Interactive `SendControl("compact")`: **untested live** to keep quota bounded
after the ~50s initial turn and ~63s summary call. Code inspection shows that it
writes `/compact\n` to the PTY; this is not evidence that agy executes it.

## Delivery

- Code: `88d34efe`; docs and retained canary: `66ab3864`; initial evidence:
  `f9e314be`. Existing native on-demand implementation needed no behavior change.
  Updated the stale 644 comment and driver documentation; regression tests now
  assert the full native invocation, empty success with unknown context, both
  CLI entry points, provider-ID routing, and provider-error propagation.
- `make lint`: PASS (existing warnings only).
- `go test ./internal/subagent/ ./cmd/harnez/`: PASS (4.728s / 33.171s).
- `make test-short`: PASS, including all packages; no 824 retry needed.
- `make install`: PASS, binary and man pages installed. `git diff --check`: PASS.
- Dedicated local review confirmed native argv and existing assertions are
  preserved; automatic thresholds remain disabled. No helpers were started.

Final live harnez check: registered the same throwaway provider conversation in
`/tmp/harnez-823-32z1ebu_/registry/canary-823.json`, with provider `agy`, model
`gemini-3.8-flash`, tier `low`, batch/completed status, context `-1`, and working
directory `/tmp/harnez-823-32z1ebu_`. Attempted:

```sh
harnez agent compact --store-dir /tmp/harnez-823-32z1ebu_/registry --name canary-823 -d /tmp/harnez-823-32z1ebu_
# exit 1; stdout empty
# Error: agent role "developer" is a leaf worker: it must not start, resume or manage agents; do the work yourself and report back; the host runs live harnez agent checks after your commit
```

The guard rejected this before launching agy; summary steps remain `[(4, 23)]`
and total steps remain 5. Raw commands, stdout/stderr, and SQLite snapshots are
retained in the scratch directory. **Pending host verification:** rerun the exact
harnez command above as host and inspect a newly saved summary, then close 823.
No role override or further live runs attempted. Issue remains blocked rather
than closed because the required end-to-end live verification cannot run from
this developer session.
