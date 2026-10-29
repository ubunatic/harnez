# 1024 — lmcoder:coder reviewer-role run fails with pi reported length and zero tokens

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 1022, 1023, lmcoder 260, lmcoder 263 (bigger window for coder)

---

## Symptom

On 2026-10-29, `harnez agent start --role reviewer --model lmcoder:coder` failed with:

```
Error: agent start "lmcoder:coder" failed: pi reported length: length
```

The run produced 0 output tokens. The same model, with `--role advisor`, returned a real one-shot review of the 1022 diff, including tool calls (git and grep). So the model and the Pi path work; the reviewer-role run is what fails.

## Suspected cause (not confirmed)

The local `coder` model has a 32,768-token context window (`~/.pi/agent/models.json`). The reviewer prompt, plus the role's instructions and the diff it was pointed at, may fill that window before the model answers. Pi then stops with `length`.

## Acceptance

- Reproduce with the reviewer role and the same prompt. Record the prompt size in tokens, if Pi reports it.
- Confirm or rule out the context-window cause: the same reviewer prompt with a shorter diff, or the advisor prompt with the reviewer's instructions, run once each.
- If the cause is the window, the fix belongs in the role prompt or in the context guard that 1023 tracks. Do not change `~/.pi/agent/models.json` or the model load.
- Add a test or a documented limit so a reviewer run on `lmcoder:coder` fails with a clear message, not a bare `length`.

## Rerun after 1023 (2026-10-29, host)
- `harnez agent start --role reviewer --model lmcoder:coder`, prompt: review `git diff 38190e98^..f27aeebd`
  (the 1022 diff; the original prompt was not kept). Compaction overrides from 1023 M1 in place.
- No `length` stop this time, but no answer either: still waiting with no output after 14 min, stopped by the
  owner at 21:56. Killing `harnez` left its `pi` child running (reparented); it had to be killed separately.
- Next: find out where the 14 minutes go (Pi session log: tool calls, compactions, prompt sizes) before
  deciding on a fix. A timeout or progress output for long synchronous lmcoder turns may belong here too.

## Rerun on the 64k window (2026-10-29 22:51, host)
- `coder` raised to 65536 by lmcoder 263; the 32k compaction override removed, so Pi defaults apply. Same prompt,
  10-minute cap (`timeout 600`).
- Result: no answer in 10 min (exit 124). Pi log `2026-10-29T21-51-15-944Z_01a189a5...jsonl`: 73 tool calls (all
  bash, 49 of them `grep`), 2 compactions (vs 13 in 14 min on 32k), peak context 49,030 tokens, 9,461 output tokens.
- Reading: the bigger window ended the compaction loop, but the model still does not converge on this review; it
  keeps searching. Next candidates: a smaller fixed prompt (1027 `plain` mode), a narrower review prompt, or a
  turn/tool-call limit. The `pi` child outlived `timeout` again but exited on its own shortly after.

## Narrow-prompt attempt (2026-10-29 23:28, host), not a test result
- Prompt: one fixed `git diff` over the 5 changed files, at most 4 more tool calls, at most 5 findings
  (scratchpad `r1024-narrow.md`).
- Run `r1024c`: `pi reported error: the engine stopped unexpectedly (exit code -15)` after 11 s; retry `r1024d`:
  `502 status code` after 15 s. Both with 0 tool calls. Proxy `/v1/models` afterwards: `coder` `live: false`.
  The server was being restarted by another agent; the run is owed once `coder` is back.

## Narrow-prompt run (2026-10-29 23:32, host, 64k window)
- Same narrow prompt (`r1024-narrow.md`: one fixed `git diff`, at most 4 more tool calls, at most 5 findings),
  session `r1024e`, `timeout 600`.
- No answer in 10 min (exit 124). Pi log `2026-10-29T22-32-28-178Z_01a189cb...jsonl`: 80 tool calls (51 `grep`,
  30 of them `grep -rn` across the repo), 2 compactions, peak 49,039 tokens, 9,723 output tokens.
- The model ignored the tool budget in the prompt; a prompt alone does not bound it. Same pattern as the
  unbounded prompt (73 calls). Next: a harness-side bound (tool-call limit per turn, or the read-only `plain`
  mode from 1027 with only the tools the task needs), not more prompt wording.
- 1029 fix confirmed live: harnez printed `interrupted by SIGTERM; provider stopped`, and no `pi` was left.

## Notes

- Out of scope: switching the proxy model. `lmcoder:strata` is not tried.
- Only Claude models may be used for the agents that run this work (OpenAI and Gemini quota is exhausted).
