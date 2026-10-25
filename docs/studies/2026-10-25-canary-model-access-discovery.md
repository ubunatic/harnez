---
kind: canary
---

# Canary: model access discovery from claude, codex and agy (2026-10-25)

Ticket 943, epic 942. Probe by advisor `c943` (luna:low), two turns, about 95 s and 79k new tokens. Read-only
except one tiny call per CLI with the made-up model `no-such-model-x`.

## Findings

| CLI | Version | List command | Stored choice | Error for an unknown or inaccessible model | Verdict |
|---|---|---|---|---|---|
| claude | 2.1.310 | none (`--model <name>`, aliases `opus`, `sonnet`) | `~/.claude/settings.json` `"model": "haiku"` | `[claude-code:unrecognized_model] {"model":"no-such-model-x",...}` then "It may not exist or you may not have access to it." | probe-only |
| codex | 0.162.1 | `codex models` exists, needs a TTY; under `script -qc` it printed control sequences and no list | `~/.codex/config.toml` `model = "gpt-6-luna"` | HTTP 400 `model_not_found`: "The 'no-such-model-x' model is not supported when using Codex with a ChatGPT account." | partly (probe; list unusable so far) |
| agy | 1.4.7 | `agy models` lists IDs: `gemini-4-argon-{low,medium,high}`, `gemini-3.8-flash-*`, Claude Opus/Sonnet 5.5 tiers, `gpt-oss-120b-medium` | not found in `~/.agy` | "model no-such-model-x is not recognized as a known model or custom model in settings", then the available list | discoverable |

Vendor APIs (docs only, no keys used): OpenAI `GET https://api.openai.com/v1/models`
([reference](https://platform.openai.com/docs/api-reference/models/list)); Google
`GET https://generativelanguage.googleapis.com/v1beta/models` ([reference](https://ai.google.dev/api/models)).
Anthropic's list endpoint was not confirmed by the probe; treat as unverified.

## Verdict for the design

- Discovery has three methods, best first: list command (agy), API list (needs keys, opt-in), probe call per
  candidate model (claude, codex). Each access fact records its method and time.
- Probe errors are distinguishable: claude `unrecognized_model`, codex `model_not_found` (names the account
  type), agy "not recognized". Claude does not separate "does not exist" from "no access".
- agy knows "custom model in settings": a user-declared model is a first-class case, matching the policy layer.
- Probe calls cost quota; discovery must be explicit (`update`), never on every start.
- Declared configs (`spec/agent.yaml`, CLI settings) show what is selected, not what the account may use.

## Gaps

`codex models` output under a pty not yet parsed (try `--json` or a wider pty); Anthropic list API unverified;
probe of a real but inaccessible model (e.g. an Argon name on a non-Pro plan) not possible on this machine.
