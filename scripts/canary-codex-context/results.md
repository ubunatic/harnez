# Codex Context Canary Results

Run `go run ./scripts/canary-codex-context -dir . -candidate features.apps=false`
from the harnez repository root. `-candidate` accepts any Codex CLI `-c key=value`
override and can be repeated. The canary runs fresh `codex exec --ephemeral --json`
sessions with the same directory and `Reply with exactly OK.` prompt. It alternates
baseline and candidate order across two pairs, checks the reply, and reports the
`turn.completed.usage` values without changing `~/.codex/config.toml`.

## Live measurement, 2026-09-16

Codex CLI 0.154.0, configured model `gpt-5.6-sol` with medium reasoning effort.
Each row below is the mean of two runs per variant. The original single-setting
runs were stable within each variant; the later combined-setting run had a
17,313–17,709 baseline range. Output was five tokens in every run.

| Directory | Candidate override | Baseline input | Candidate input | Delta |
|---|---|---:|---:|---:|
| `/tmp` | `features.apps=false` | 14,771 | 12,301 | -2,470 (-16.7%) |
| harnez root | `features.apps=false` | 17,313 | 14,843 | -2,470 (-14.3%) |
| `/tmp` | `features.plugins=false` | 14,771 | 13,018 | -1,753 (-11.9%) |
| `/tmp` | disable only `story` skill | 14,771 | 14,744 | -27 (-0.2%) |
| harnez root | `features.apps=false`, `features.plugins=false` | 17,511 | 14,635 | -2,876 (-16.4%) |

The skill run used:

```sh
go run ./scripts/canary-codex-context -dir /tmp \
  -candidate 'skills.config=[{path="/home/uwe/.codex/skills/story/SKILL.md",enabled=false}]'
```

`input_tokens` is Codex's reported first-turn input total; `cached_input_tokens`
is reported separately and is not added to it. This is a measurement of the
request made by a minimal prompt, not a breakdown of every context component or
a prediction of savings on longer tasks. Cache hits varied between runs, and
the later combined run also showed baseline input variation. The broad toggles'
savings overlap; do not add their separate deltas. The canary tests
configuration effects; it does
not verify that a disabled capability is unneeded in real work. Local hooks can
still run during `codex exec` even though the session is ephemeral.

Disabling Apps produced the largest tested saving, but removes connector
integrations. Disabling plugins also removes plugin capabilities. A single
skill contributed little to this setup's first-turn cost, so selective skill
removal alone is unlikely to match the broad integration toggles. Measure more
specific plugin or tool controls before choosing a permanent debloat preset.

Codex configuration keys: [OpenAI Docs configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).
