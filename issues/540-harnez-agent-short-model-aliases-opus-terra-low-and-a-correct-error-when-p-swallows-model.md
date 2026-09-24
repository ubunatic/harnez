# 540 — harnez agent: short model aliases (opus, terra:low) and a correct error when -p swallows --model

**Status**: Open
**Priority**: P1
**Severity**: Medium
**Category**: Usability / Agents
**Related**: [[449-add-spec-driven-agent-chat-model-selection-and-aliases]], [[539-agents-in-other-projects-don-t-know-harnez-agent-or-model-names-like-terra-low]]

---

## Observed (2026-09-24, HEAD 7f43412+)

Aliases resolve inconsistently:

| `--model` | Result |
|---|---|
| `terra`, `codex:terra` | resolves (codex terra) |
| `terra:low` | rejected: unknown model |
| `opus` | rejected: "unknown or ambiguous" (both `agy:opus:low` and `claude:opus:low` exist) |
| `agy:gemini-3.7-flash:low` (the MODEL name shown in `harnez stats`) | rejected; only `agy:flash37:low` works |

Misleading error: `harnez agent -p --model agy:flash37:low "text"` fails with
`model is now --model <spec>; to send this text literally put it after --` (`cmd/harnez/agent.go:143`).
The real cause is that `-p` takes the next token (`--model`) as its prompt value, so `--model` stays
empty and the model name becomes the first prompt word.

## /goal

1. Short aliases resolve to one spec, as defined in `spec/agent.yaml` (not in Go code, see docs/Spec.md):
   - a bare model gets a default provider and tier: `opus` → `claude:opus:low`, `sonnet` →
     `claude:sonnet:low`, `haiku` → `claude:haiku:low`, `terra` → `codex:terra:low`, `luna`, `sol`,
     `astra` → `codex:<m>:low`, `flash37` → `agy:flash37:low`
   - `<model>:<tier>` without a provider: `terra:low`, `opus:med`
   - the full model names shown in `harnez stats`/`agent list` (e.g. `agy:gemini-3.7-flash:low`,
     `codex:gpt-5.6-terra:med`) are accepted as well.
   Ambiguous bare names use the spec's preferred provider; `harnez agent models` shows the alias column.
2. When `-p`/`--prompt` receives a value that starts with `--`, fail with an accurate message, e.g.
   `-p needs prompt text but got flag "--model"; put -p last or use -- "<text>"`. Keep the
   "model is now --model" hint only for the genuine old-style `harnez agent <model> <text>` form.

## Notes

- 449 (chat model selection and aliases) overlaps with item 1. Check what it already delivered and
  close or narrow one of the two.
- Tests: an alias table test driven by the spec, the ambiguity rule, and the `-p --model` error.
- 539 benefits: "ask terra:low …" from other projects then works verbatim.
