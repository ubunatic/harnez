# 1024 — lmcoder:coder reviewer-role run fails with pi reported length and zero tokens

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 1022, 1023, lmcoder 260

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

## Notes

- Out of scope: switching the proxy model. `lmcoder:strata` is not tried.
- Only Claude models may be used for the agents that run this work (OpenAI and Gemini quota is exhausted).
