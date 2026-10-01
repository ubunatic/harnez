# Claude Resume Compaction (Issue 673)

Study of why `harnez agent resume` could not resume a Claude worker after a long turn, how it was
found, and what fixed it. Sprint of 2026-10-01, found during issue 656 M1.

## Symptom

```
Error: refusing to send resume prompt: compaction completed without a verified context drop (before 441684, after 0 full input tokens)
```

Every Claude worker whose turn had many tool steps became impossible to resume. In the 656 sprint this
cost two developer sessions (`dev-656-sonnet` at 849k, `dev-656-fix` at 441k). Each had to be replaced
by a fresh session that was briefed from the ticket.

## Three Stacked Bugs

The error had three causes on top of each other. Each fix revealed the next one, and only live checks
on a stuck worker exposed them; unit tests passed at every stage.

| # | Bug | Evidence | Fix |
|---|---|---|---|
| 1 | Context size read from the turn total | `--output-format json` has only `result.usage`, which sums every model call of the turn. Canary with 4 calls: last call 16,854, turn total 66,443. Real context of `dev-673` before compact was 44,835, stored was 346,607 | stream-json with `--verbose`; context = last main-thread assistant call (input + cache_read + cache_creation), or `compact_boundary.post_tokens`, else unknown (-1) (1da8887f) |
| 2 | `/compact` stream parsed to 0 | After `compact_boundary`, Claude emits synthetic `assistant` events with all-zero usage and `model: "<synthetic>"`; the result text is empty and the ack `Compacted` is only in a `user` event's `<local-command-stdout>` | skip zero-usage events (033f0f95); read the local-command stdout as the acknowledgement (650a5022) |
| 3 | `/compact` never ran | `ClaudeDriver.Compact` passed `Model{}`, so harnez ran `claude --model ""`. Claude answered `result` subtype **success** with text `Error during compaction: API Error: 400 model: String should have at least 1 character` | omit empty `--model`/`--effort`; map `Error during compaction` to an error (783a2aa2) |

Bug 3 means no harnez compaction of a Claude session had ever worked. Bug 1 made harnez try one on
nearly every resume after real work, so the latent bug 3 surfaced as a hard block.

## Event Sequence of a Real `/compact` (stream-json)

```
system/init
system/compact_boundary   compact_metadata.pre_tokens=48922, post_tokens=4745
user                      "This session is being continued ..." (summary)
assistant x2-3            model "<synthetic>", usage all 0
user                      "<local-command-stdout>Compacted </local-command-stdout>"
result                    subtype success, result "", usage all 0
```

## How It Was Found

- Developer canary (short haiku run) for bug 1. Correct, but it didn't cover a real resume.
- Host resumed stuck workers after each fix (4 resumes per check). Two checks failed: "after 0", then
  "after -1".
- Host ran harnez's command by hand on the stuck session and fed the captured stream to `parseClaude`
  in a throwaway test. The parser returned 4,745 and verification passed, so the fault had to be in the
  arguments. Repeating with harnez's **exact** args (`--model ""`) showed the 400.

Lesson: a canary must use the production command line verbatim, not a hand-written equivalent. The
first probes used `--model sonnet` and hid bug 3.

## Verification

`dev-656-fix` (stored 441,684 from the old parser), resumed twice after 783a2aa2:

```
[20:04:48 compact] verified /compact at 441.7k context tokens   -> ok, context 20903
(second resume, no compaction)                                  -> ok, context 20966
```

`dev-673b` resumed three times after turns of 224k and 100k totals without any compaction.

## Open Points

- The compact now runs on Claude's default model, not the session's model, which may cost more per
  compaction. Passing the session model to `Compact` would avoid that.
- Sessions stored by the old parser compact once on their next resume (expected, verified above).
