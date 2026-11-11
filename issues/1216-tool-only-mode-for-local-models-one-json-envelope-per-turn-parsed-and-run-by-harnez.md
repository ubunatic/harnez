# 1216 — Tool-only mode for local models: one JSON envelope per turn, parsed and run by harnez

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 962, 1215

---

## 1. Problem & Motivation

Owner idea (2026-11-10): local models (lmcoder/qwen) are weak at free agentic chat. In a tool-only mode the
model may only answer with one JSON envelope per turn, `{"tool":"<name>","args":{...}}` or
`{"final":"<text>"}`, which harnez parses and executes. 962 (isolated mode, no tools at all) is the opposite
case.

## 2. Findings (Jules leads, checked)

Experiment `reports/jules/tool-only-envelope.md` (GH#175, code not merged; host reran its test at the PR
head, pass): a lenient parser (first `{`, last matching `}`) handles prose around the JSON better than a
strict one. Risk: braces in the prose can select the wrong object.

Ideas `reports/jules/tool-only-plug-in.md` (GH#176, checked at 14dd2c48, 4 of 5 references held):

1. A `ToolOnly` field on `Model` (`internal/subagent/driver.go:133`) and the spec alias, so drivers see it
   in `RunOptions.Model`.
2. Schema: a `tool_only` boolean on model entries (`spec/schemas/agent.schema.json`, the `models` key is at
   line 248, not the cited 200).
3. Malformed reply: feed the parse error back as the next prompt, around `t.Driver.Run(ctx, t.Opts)`
   (`internal/subagent/resume.go:80`), with a retry cap.
4. Show the mode in `ModelEntry` (`internal/subagent/driver.go:359`) for `agent models`.
5. Unverified, likely wrong: a `--tool-only` flag for the `pi` CLI in `piRunArgs`
   (`internal/subagent/pi.go:113`); nothing shows that pi has such a flag. The envelope rules more likely go
   into the prompt.

Open: which tools the envelope may name, and who executes them (harnez itself or the provider CLI).

## 3. Goal

/goal A model entry with `tool_only: true` in `spec/agent.yaml` makes harnez send the envelope rules,
parse each reply with the lenient parser, run the named tool from an allowlist in spec, and feed malformed
replies back up to a capped number of retries; tests use a real captured lmcoder/qwen reply. Stop and ask the
owner on the tool allowlist. On a loop or no progress, clear the goal and write a failure report here.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| experiment | jules-experiment-tool-only-envelope | 18278899567407530536 | github #175 (closed) | done: works, test rerun by host |
| ideas | jules-ideas-tool-only-plug-in | 3116594122828967457 | github #176 (closed) | done: 4/5 refs hold |
