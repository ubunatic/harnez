# 1023 — Adopt lmcoder findings for Pi with local 32k models: compaction settings, capped reads and spill, context guard

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: 969, 403, 725, 771, 716

---

## 1. Problem & Motivation
Pi sessions with a local 32k-context model (Strata coder on an RTX 3060, `n_ctx` 32768) failed with replies
cut off after one token (`finish_reason: length`). The lmcoder repo (epic 250, `/home/uwe/projects/lmcoder`)
found the cause and a working mitigation in its agent container. harnez should adopt the findings wherever it
launches or configures Pi for local models, so the host Pi benefits too.

`/goal` Record the lmcoder findings here, pick what harnez should do for local small-context Pi models, and
file or link the child work; or stop and report when blocked on a user decision. Circuit breaker: on a loop or
no progress, clear the goal and write a failure report. Re-verify every claim below against live code and the
lmcoder tickets before acting (they were found on 2026-10-29).

## 2. Technical Specification / Findings
Sources: `lmcoder/docs/studies/2026-10-29-epic-250-sprint.md` (summary),
`lmcoder/docs/studies/2026-10-29-epic-250-group-a-research.md` (research), lmcoder tickets 248 to 258.

**Root cause (reproduced and fixed in two runs).** Pi compacts only messages older than `keepRecentTokens`
(default 20000), triggered above `contextWindow - reserveTokens` (default 16384). With a 32768 window and a
conversation of about 20k estimated tokens there is nothing old enough, and `prepareCompaction` returns nothing,
so compaction is skipped without an event (`compaction.js:628-643`, `agent-session.js:2533-2535`, Pi 1.4.0).
Pi then sends `max_tokens = min(16384, 32768 - prompt - 4096)`, which is 1 at a 29k prompt
(`pi-ai simple-options.js:3-9,23`): the one-token `<tool_call>` reply. Reasoning tokens were 0 and not involved.
Pi's defaults assume a window of 100k or more.

**Fix that worked (in the container).** Pi `settings.json`:
`{"compaction":{"reserveTokens":14000,"keepRecentTokens":8000}}`. Same stress prompt: default settings ended with a
`length` stop at 29,199 prompt tokens; with the settings there was no `length` stop in 20 minutes (peak 19,622).
Per-model overrides exist: `compaction.modelOverrides["llamacpp/strata"]` (`settings-manager.js:643-658`).

**Pi extension API (verified from source).** `tool_call` can block or mutate input of the built-in `read`;
`registerTool({name:"read"})` overrides it (`examples/extensions/tool-override.ts:68-115`); `tool_result` can
replace content (`types.d.ts:985,1110`); `ctx.getContextUsage()` and `ctx.compact()` exist. Extensions load from
`<agent-dir>/extensions/`, `.pi/extensions/` or `pi --extension <file>`. Pi's built-in tool output truncation default
is 2000 lines or 50 KB (`truncate.js:4`), about 12k tokens, too big for 32k.

**Guard extension (lmcoder `scripts/agent-canaries/multi/extensions/context-guard.ts`).** Capped `read` override
(default 150 lines / 6000 chars, env `CONTEXT_GUARD_*`), spill of oversized other tool results to a file with a
partial-output notice and bounded lookup, caps halved above 60% context, "stop reading, answer now" notice above 70%,
images delegated to Pi, binary detection, streamed reads. Reviewed by `opus:med`. This is the Pi read hook that
harnez issue 403 was blocked on: it is feasible.

**What works.** The compaction settings; capped reads with spill files; llama.cpp prompt-prefix caching for repeated
questions about one document (later questions reused 15,041 prompt tokens, 4 to 7 s instead of 28 s); the realistic
goself replay (two questions, 44 s, peak 9,935 tokens, correct).

**What does not work / pitfalls.**
- The guard alone did not change outcomes: the unguarded baseline with compaction also passed the realistic replay,
  so its benefit is unproven. The compaction setting is the proven fix.
- Reading several large docs in full (stress, 5 docs) never completes on 32k: 64 tool calls, 14 compactions, timeout.
- A lookup that reads the whole spill file recreates the overflow; every lookup needs its own cap.
- Strata gets structural questions wrong (heading lists include second-level headings) and once claimed a full read
  it had not done, both in Pi and in a tool-less reader.
- A silent skip: compaction produces no event when nothing is old enough, so logs show nothing.
- Pi's `--no-extensions` does not disable explicitly loaded `-e` extensions.
- Alias routing detail: on x600, `strata` reaches the loaded model on port 8080; do not assume other aliases do.

## 3. Implementation & Verification Plan
Decide what harnez does; candidate items (each its own child ticket if accepted):
1. When harnez launches or configures Pi for a local model, derive `compaction.reserveTokens` and
   `keepRecentTokens` from the model's context window instead of Pi's defaults (relates to 969). Rule of thumb from
   the finding: keep recent tokens at about a quarter of the window and reserve about 40%.
2. Warn when a Pi session's context window is below about 64k and compaction settings are at Pi's defaults.
3. Ship or reference the context-guard extension for Pi (relates to 403, 716, 725, 771); decide whether it lives in
   harnez or stays in lmcoder.
4. Log a visible event when Pi compaction is skipped because nothing is old enough.

Verification: reproduce with a 32k local model and default settings (length stop), then with the derived settings
(no length stop), and re-run the lmcoder realistic replay (`scripts/strata-context-replay.py`, ticket 255).
