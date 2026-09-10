<!-- harnez:topic: Advisor session reuse, cache evidence, and cross-agent dispatch design -->

# Advisor Session Reuse and Cross-Agent Dispatch

**Scope**: Whether resuming a compatible advisor session saved tokens in the
Astra example from issue 303, and a design for applying safe reuse across
Claude, Codex, Gemini/AGY, and Prime Agent.

**Accessed**: 2026-09-10

## Finding

The Astra reuse produced strong cache evidence, but not a measured fresh-session
comparison. The resumed assessment turn recorded `155,900` input tokens, of
which `145,152` were cached: a **93.1% cache-hit rate** with `10,748` uncached
input tokens. It produced `1,976` output tokens, including `931` reasoning
tokens.

This proves that most of that turn's input was served from the provider's cache.
It does not prove that resuming saved exactly `145,152` tokens or any specific
amount of money: a fresh session might still reuse stable system-prompt cache
entries, and the available record has no uncached fresh-session control run or
dollar-cost field. The correct result is **measured cache reuse, unmeasured
counterfactual savings**.

## Evidence from the Astra session

The local Codex rollout record for session
`01a08a2e-99b6-7e42-9215-d38eb4f38546` contains `token_usage_record` entries
with input, cached-input, output, reasoning, and total token fields. The final
resumed advisor turn was recorded at `2026-09-10T10:02:11Z`:

- Input: `155,900`
- Cached input: `145,152`
- Uncached input: `10,748`
- Cache-hit rate: `145,152 / 155,900 = 93.1%`
- Output: `1,976`
- Reasoning output: `931`

The same record contains cumulative thread totals, but those totals mix the
original advisor work, later resumed work, and tool calls. They are useful for
accounting, not for claiming a fresh-session saving. The record has rate-limit
metadata but no price or billed-cost amount.

The evidence supports these statements:

- **Measured**: the resumed Astra request had a 93.1% cached-input share.
- **Provider-reported**: input, output, reasoning, and cache fields were
  present in Codex's local rollout record.
- **Unknown**: tokens, latency, and money avoided versus a fresh advisor
  session.

The evidence does not support saying that session resumption always saves
tokens. Resumption can restore a large history and increase the total input
sent; cache accounting and context restoration are separate measurements.

## Current tool inventory

The local environment was inspected using installed CLI help, Harnez's
configuration, and the managed target directories.

- **Claude Code** is installed. Its CLI exposes `--resume`, `--continue`,
  `--fork-session`, `--effort`, `--model`, permission controls, and background
  session management. Resuming can retain the original session ID; forking is
  an explicit alternative. The CLI help exposes session control but no stable
  local cost report that Harnez can consume generically.
- **Codex** is installed. `codex resume` accepts a session ID and optional new
  prompt; the CLI exposes model and execution controls. The local rollout JSONL
  records input, cached input, output, reasoning, and total tokens, which makes
  Codex the strongest measured example in this environment. The current CLI
  help does not expose a direct `--effort` flag; reasoning selection may be
  controlled by the host or configuration instead.
- **Gemini CLI / AGY** is not installed under the `gemini` name here, while
  `agy` is installed. AGY exposes `--continue`, `--effort low|medium|high`,
  `--model`, and project selection. Its help does not expose a stable usage or
  cache report in this environment. Gemini CLI's current upstream documentation
  describes project-scoped session persistence, `--resume` by latest/index/ID,
  `--list-sessions`, and saved input/output/cached token statistics. That
  capability must be treated as conditional on the Gemini CLI actually being
  installed and authenticated.
- **Prime Agent** has a managed target at `~/.prime/agent`, but no
  `prime-agent` executable was found on this PATH during the investigation.
  Harnez can describe Prime support, but should not claim it can discover or
  resume Prime sessions until a supported executable or API adapter is present.

Sources: [Gemini CLI session management](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/session-management.md), [Gemini CLI reference](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/cli-reference.md), and the local `claude`, `agy`, and `codex resume --help` output captured during this investigation.

## Proposed interface

The user-facing entry point should be a cross-agent skill named `/advisor`, with
`reuse` as its first subcommand. This makes the workflow callable from each
supported harness using the same slash-command shape:

```text
/advisor reuse [--agent claude|codex|agy|gemini|prime]
              [--session ID] [--effort low|medium|high]
              [--fresh] [--json] [--dry-run]
```

The skill should delegate compatibility and evidence decisions to one Harnez
command with per-tool adapters. The corresponding host-side contract is:

```text
harnez advisor reuse [--agent claude|codex|agy|gemini|prime]
                     [--session ID] [--effort low|medium|high]
                     [--fresh] [--json] [--dry-run]
```

The `/advisor` skill should produce a launch plan or structured result rather than
silently starting an interactive agent. The result identifies:

- selected agent and adapter;
- selected or newly created session ID, when available;
- compatibility decision and each failed check;
- chosen model and reasoning setting;
- resume or fresh-start operation;
- evidence level (`measured`, `reported`, `estimated`, or `unknown`);
- token/cache/latency/cost fields when the adapter can obtain them; and
- fallback reason and the command needed to continue.

The default should be `--effort low` for bounded advisory work where the tool
supports an effort control. The user must be able to request another level, and
the result must say when the selected tool cannot honor that setting.

## Compatibility identity

A prior advisor session is reusable only when all required identity fields match
or the user explicitly overrides a non-critical difference:

- agent family and CLI/API version;
- model family and model version;
- reasoning or effort setting, if part of the task contract;
- project root and repository identity;
- relevant instruction fingerprint (`AGENTS.md`, managed skill, and selected
  advisor prompt hashes rather than their private contents);
- permission, sandbox, and tool-availability profile;
- declared task scope and advisor role;
- session age, validity, and resumability; and
- required evidence capabilities, such as token or cache reporting.

The task identity should be a caller-declared scope such as “assess Harnez
plugin system,” not a fuzzy similarity score over private transcript text. A
recent session from the same provider is insufficient. The command must reject
reuse when the project, permissions, model, role, or instructions are
incompatible, or when it cannot determine the difference safely.

## Per-tool adapter behavior

Each adapter should implement four operations:

1. discover candidate sessions;
2. inspect identity and usage metadata;
3. resume a validated session or start a fresh one; and
4. collect post-run evidence without retaining the transcript.

The initial capability matrix is:

- **Claude**: discover/resume by session ID; optionally fork; select model and
  effort; usage and cost evidence is adapter-dependent and should be marked
  unknown when unavailable.
- **Codex**: discover/resume by session ID; preserve or fork according to the
  CLI/API operation; collect local rollout token fields; classify money and
  fresh-baseline savings as unknown unless a provider billing record exists.
- **AGY/Gemini**: use AGY's continue behavior where the installed CLI exposes
  it; use Gemini's project-scoped resume and session listing when Gemini CLI is
  installed; select effort only where supported; collect token/cache fields only
  from machine-readable output or session metadata.
- **Prime Agent**: remain unsupported until its session API or executable is
  available; fall back to a fresh supported advisor or report unavailable.

An adapter must never emulate resumption by copying a transcript into a new
prompt and then label the result as a resumed session. That is a fresh session
with copied context and must be reported separately.

## Fresh-session fallback

The command must start fresh when any of these conditions holds:

- no candidate session exists or the requested session cannot be opened;
- agent, model, project, permissions, instructions, or task scope mismatch;
- the session is expired, corrupted, already active in an incompatible mode, or
  beyond the tool's context/session limit;
- the requested effort or output mode cannot be honored;
- the adapter cannot determine whether the candidate is safe to reuse; or
- the user passes `--fresh`.

Fallback should retain the compatibility decision and reason in the result. It
must not silently reuse the nearest recent session after a failed check.

## Evidence and durable record

Persist a small run record, for example under an XDG state directory or a
project's Harnez-local state, containing only:

- run ID and timestamp;
- agent, adapter, model, effort, project identity, and session ID;
- compatibility inputs as hashes or normalized labels;
- reuse/fresh decision and fallback reason;
- provider-reported usage fields;
- evidence classification; and
- tool/version metadata.

Do not persist prompts, transcripts, credentials, or arbitrary tool output in
this record. Redact session IDs if a provider treats them as sensitive, or store
them only in a user-owned local state file with restrictive permissions.

The evidence classifier should follow these rules:

- **Measured**: a resumed run and a comparable fresh control run use the same
  task, model, project, and settings, with provider token/cache measurements.
- **Reported**: the provider reports usage for the resumed run, but no matched
  fresh control exists.
- **Estimated**: Harnez derives a context-size or cache estimate without a
  provider-confirmed comparison.
- **Unknown**: the adapter cannot obtain trustworthy usage data.

Never report “saved tokens” or “saved cost” for `reported`, `estimated`, or
`unknown` evidence without explicitly labeling it as an estimate or gap.

## Verification plan

The implementation should use deterministic adapter fixtures for:

- compatible session selection;
- project, model, permission, instruction, and task-scope mismatches;
- missing, expired, corrupt, or already-running sessions;
- unsupported resume or effort controls;
- explicit fresh mode;
- measured versus reported, estimated, and unknown evidence; and
- redaction of credentials, prompts, and transcripts from durable records.

An integration canary should exercise one real resumable Codex or Claude
session and verify that the output records the session ID, compatibility
decision, effort, and available usage fields. A separate fresh control run is
needed before claiming savings. For Gemini/AGY and Prime, tests should verify
the unsupported or conditional adapter paths in environments like the current
one.

## Recommendation

Implement the first version as a **session-selection and evidence command**,
exposed through the `/advisor reuse` skill. Keep launching
and provider-specific resumption in adapters, keep compatibility decisions
strict, default bounded advisory work to low effort, and make unknown savings
the normal honest result when no matched control exists.

Do not begin with autonomous session mining, transcript similarity, automatic
cross-project reuse, or a universal cost calculator. Those features would make
the system appear convenient while weakening task isolation and making its
savings claims difficult to reproduce.
