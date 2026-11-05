# `harnez agent models --json`

Machine-readable per-model view for tools such as taia (issue 1122). The output is a JSON array; the
contract is `spec/schemas/agent-models-row.schema.json`, checked by a test against real output.
`harnez models --json` is the same command.

The command reads cached state only (the usage store and the local block file). It never starts a
collector, so `--offline` is accepted and is always in effect. Quota evidence can therefore be `stale`
or `unknown`; ask `harnez usage` to refresh it.

## Row fields

| Field | Meaning |
|---|---|
| `spec` | Model spec, e.g. `codex:luna:low`. |
| `provider` | Login id of the provider, e.g. `codex`, `agy`. |
| `model` | Provider model name (a string). The former `model` object is now `model_detail`. |
| `quota.state` | `available`, `exhausted`, `stale` or `unknown`; the same value as the AVAILABILITY text column. |
| `quota.remaining_percent` | Smallest remaining share over the model's quota windows (0-100), or `null` without window data. |
| `quota.reset_at` | RFC3339 time the quota returns, set when `state` is `exhausted`, else `null`. |
| `quota.age_s` | Seconds since the newest reading. |
| `quota.scope` | `model` when the evidence is per model (a local block, AGY pools), else `provider`. |
| `startable` | `true` unless quota is exhausted (including a block) or the model is disabled by policy. |
| `startable_reason` | Present when not startable: `quota exhausted`, `blocked until <RFC3339>`, `model disabled by policy`. |
| `entitlement.state` | `verified`, `unverified` or `denied`. Separate from quota; being in the spec never means access. |
| `entitlement.source` | Where the state came from: `spec` (nothing verified) or `model-policy` (disabled by policy). |
| `entitlement.checked_at` | RFC3339 time of the check, or `null`. |
| `blocked_until`, `block_reason` | Present only while a local block is active. |

Other fields (`batch`, `effort`, `cost`, `eff`, `skills`, `roles`, `use`, `source`, `model_detail`) keep
their former meaning. Today `entitlement.state` is `unverified` or `denied`; `verified` is reserved for
a future provider-side check.

## Blocking a model

```
harnez models block <spec|provider:model> --until <RFC3339|duration> [--reason text]
harnez models unblock <spec|provider:model>
```

A block is a local record in `$XDG_STATE_HOME/harnez/model-blocks.json` (default
`~/.local/state/harnez/model-blocks.json`), keyed by provider and model name. While active, quota
availability reports the model as `exhausted`, so `agent start`/`resume` refuse it unless
`--allow-exhausted-quota` is given. Expired blocks are ignored and dropped on the next write.

`agent start` and `agent resume` record a block themselves when a failed turn carries a known quota error
(AGY "Individual quota reached", Codex usage limit). The block lasts until the reset time the error
names, else `default_duration`. Patterns and the default live in `spec/model_blocks.yaml`
(schema `spec/schemas/model_blocks.schema.json`). The reset wording in those patterns is unverified
against live provider output.
