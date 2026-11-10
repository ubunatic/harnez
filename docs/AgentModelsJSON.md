# `harnez agent models --json`

Machine-readable per-model view for tools such as taia (issue 1122). The output is a JSON array; the
contract is `spec/schemas/agent-models-row.schema.json`, checked by a test against real output.
`harnez models --json` is the same command.

The command reads cached state only (the usage store, local block file, and model access observations). It never starts a
collector, so `--offline` is accepted and is always in effect. Quota evidence can therefore be `stale`
or `unknown`; ask `harnez usage` to refresh it.

## Row fields

| Field | Meaning |
|---|---|
| `spec` | Model spec, e.g. `codex:luna:low`. |
| `provider` | Login id of the provider, e.g. `codex`, `agy`. |
| `model` | Unchanged object `{Provider, Name, Tier, Effort}` that existing consumers read (ticket 983). |
| `model_name` | The plain provider model name as a string (same as `model.Name`). |
| `quota.state` | `available`, `exhausted`, `stale` or `unknown`; the same value as the AVAILABILITY text column. |
| `quota.remaining_percent` | Smallest remaining share over the model's quota windows (0-100), or `null` without window data. |
| `quota.reset_at` | RFC3339 time the quota returns, set when `state` is `exhausted`, else `null`. |
| `quota.age_s` | Seconds since the newest reading. |
| `quota.scope` | `model` when the evidence is per model (a local block, AGY pools), else `provider`. |
| `startable` | `true` unless quota is exhausted (including a block) the model is disabled by policy, or account access is unsupported. |
| `startable_reason` | Present when not startable: `quota exhausted`, `blocked until <RFC3339>`, `model disabled by policy`, `unsupported for this account`. |
| `entitlement.state` | `verified`, `unverified` or `denied`. Separate from quota; being in the spec never means access. |
| `entitlement.source` | Where the state came from: `spec` (nothing verified) or `model-policy` (disabled by policy), or `model-access` (observed turn). |
| `entitlement.checked_at` | RFC3339 time of the check, or `null`. |
| `blocked_until`, `block_reason` | Present only while a local block is active. |

Other fields (`batch`, `effort`, `cost`, `eff`, `skills`, `roles`, `use`, `source`) keep
their former meaning. A successful Codex turn records verified access; a matching account-unsupported failure records denied access. The latest observation wins. Text output shows `unsupported for this account`. Observations are stored in `$XDG_STATE_HOME/harnez/models/access.yaml` (default `~/.local/state/harnez/models/access.yaml`).

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
(AGY "Individual quota reached", Codex usage limit). Every `agent resume` also refuses a session whose
model is blocked, for any provider, unless `--allow-exhausted-quota` is given.

The block lasts until the reset time the error names, else `default_duration`. Patterns and the default
live in `spec/model_blocks.yaml` (schema `spec/schemas/model_blocks.schema.json`). Each pattern is a regex
anchored at the start of the driver's error text and spells the provider's own error shape, so an ordinary
failure that merely mentions "usage limit" never blocks. The Codex reset wording is unverified against live
provider output; the AGY pattern is tested against `internal/subagent/testdata/agy_quota_error.json`.

The block file is changed under an exclusive file lock with unique temp files, so concurrent `block` and
`unblock` calls do not lose updates.
