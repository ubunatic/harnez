# 1217 — Per-model-family start/resume preamble from spec

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1068, 1215, 1216

---

## 1. Problem & Motivation

Owner idea (2026-11-10): the text harnez puts in front of an agent's first prompt on start and resume is
the same for every model family. Claude, Codex/GPT, Gemini/agy and local qwen/lmcoder follow different
instructions best, so each family should get its own preamble overlay, kept in `spec/`.

## 2. Findings (Jules ideas, checked)

`reports/jules/preamble-per-family.md` (GH#178, checked at af802ef3, 5 of 5 references held):

1. The preamble is assembled in `WithProtocol(prompt, planFirst, role, inferred)`
   (`internal/agent/core.go:31`), starting from `pre := protocolPreamble` (line 32); it gets no model or
   provider today.
2. `internal/agent` already imports `internal/subagent`, so a spec lookup there adds no import cycle.
3. Plug-in: a `preambles` map keyed by provider in `spec/agent.yaml` (top level, near `default_role` at
   line 340) plus schema, parsed into `agentSpec` (`internal/subagent/agentspec.go:42` area), read by a
   function like `RoleRules` (`agentspec.go:722`), and appended to `pre` when the provider has an entry.
4. Callers of `WithProtocol` must pass the provider; an unknown or empty provider keeps today's text.

Open: the preamble text per family (owner or measured), and whether resume repeats it.

## 3. Goal

/goal `spec/agent.yaml` holds an optional preamble per provider, schema-checked, and `WithProtocol`
appends it on start (and on resume if decided), with a test per provider and one proving a provider
without an entry gets the unchanged text. Stop and ask the owner for the preamble texts. On a loop or no
progress, clear the goal and write a failure report here.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| ideas | jules-ideas-preamble-per-family | 7652669206978174282 | github #178 (closed) | done: 5/5 refs hold; the session showed FAILED, but the PR was delivered |
