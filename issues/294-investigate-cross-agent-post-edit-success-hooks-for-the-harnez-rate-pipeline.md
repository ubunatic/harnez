# 294 — Investigate cross-agent post-edit success hooks for the harnez rate pipeline

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [124 — automatic internal-tool counts](124-posttooluse-internal-tool-call-auto-capture.md), [181 — failure-only rating policy](181-narrow-harnez-rate-to-failure-cases.md), [271 — AGY hook decommission](271-decommission-agy-hooks-pretooluse-interception-in-favor-of-guarded-bash-path-shim.md), [273 — optional AGY hook restoration](273-restore-old-agy-pretooluse-hook-as-opt-in-configuration-option.md), `docs/HookRewritePattern.md`, `docs/Canary.md`, `docs/CLIDesign.md`

---

## 1. Problem & Motivation

Determine whether Codex, AGY, and Claude support PostEdit, PostUpdate, or
PostWrite hooks that can report successful operations; where supported,
evaluate adding those success events to the `harnez rate` pipeline.

Today, agents manually rate failures or unexpected outcomes. Automatic
evidence that file operations completed could fill the success-observation
gap without adding agent-issued rating calls after every edit. A successful
tool execution does not establish that an edit achieved its intended result.

Duplicate search with `harnez find` identified issue 124 as closely related:
it proposes generic automatic internal-tool counts, initially through Claude.
This ticket owns the three-platform capability investigation and the narrower
semantics of successful edit/write/update events. Reuse or coordinate with 124's
event ingestion instead of implementing two competing collectors. Issue 177's
post-edit build checks are separate; a completed write is not a passing build.

## 2. Findings and Remaining Uncertainty

Inspected on 2026-09-09: `codex-cli 0.153.4`, AGY `1.1.28`, and Claude Code
`2.1.266`. Documentation establishes the following candidate mechanisms; no
live hook-fire canary was run while filing this issue. None of the inspected
event lists documents literal `PostEdit`, `PostUpdate`, or `PostWrite` events.
Use the platform's actual lifecycle event and tool matcher.

- **Claude Code — documented support.** `PostToolUse` follows successful tool
  calls; `PostToolUseFailure` covers execution errors. Match `Edit|Write` for
  native file operations; verify any additional update tools individually.
  Tool names and call IDs allow attribution. Failed validation, denial, or
  cancellation paths need separate coverage checks. See the official
  [hooks reference](https://code.claude.com/docs/en/hooks#posttooluse).
- **Codex — documented support, installed behavior needs a canary.**
  `PostToolUse` covers `apply_patch`, with `Edit` and `Write` matcher aliases;
  the payload still identifies `apply_patch`. It includes `tool_use_id`,
  `tool_input`, and tool-specific `tool_response`. The event also runs for
  nonzero Bash exits, so its occurrence alone is not success evidence.
  Probe patch failures and result encoding before choosing a success test.
  Official docs describe `[[hooks.PostToolUse]]` and standalone hook files;
  harnez's `internal/codex/hooks.go` currently generates named
  `[hooks.harnez]` configuration. Validate the installed schema and trust
  behavior before extending that writer. See
  [Codex hooks](https://learn.chatgpt.com/docs/hooks).
- **AGY — documented in the installed vendor bundle; success attribution is
  incomplete.** `~/.gemini/antigravity-cli/builtin/skills/agy-customizations/docs/hooks.md`
  documents `PostToolUse` after tool completion. Its post payload lists
  `stepIdx`, common `conversationId`/`workspacePaths`, and `error` when the
  tool failed; it expects `{}` on stdout. Matchers use lowercase step types
  with the `CORTEX_STEP_TYPE_` prefix removed. The post contract does not
  document `toolCall.name` or a result body: probe actual edit/write step
  names and successful/failed payloads. A per-tool matcher or correlation
  with `PreToolUse` may be needed; do not assume Claude's payload shape.
  This CLI evidence does not establish behavior in other Antigravity clients.

Relevant repository constraints:

- `config.yaml` installs Claude's `PreToolUse` shell interceptor. Existing
  `harnez exec` rows already cover shell execution; shell commands can also
  write files, but must not be counted again as native edit events.
- `internal/claude/apply.go` removes AGY's old managed hook entry under issue
  271. `internal/agy/hooks.go` remains present, but its existence does not
  mean hooks are currently installed. A future success observer must coexist
  with the guarded shell shim and survive apply/cleanup without restoring
  the command-rewrite UI behavior that motivated decommissioning.
- `cmd/harnez/rate.go` currently writes scored `internal` rows, or unscored
  `heartbeat` rows with `--ok`. Neither is an established per-operation
  machine-success contract. `internal/telemetry/types.go` has nullable scores
  and exit codes but no upstream tool-call ID field. Check schema, ingestion,
  queries, export, and reminder semantics before selecting a representation.

## 3. Proposed Scope and Acceptance Criteria

- [ ] Record a capability result per platform with version, exact event and
  matcher, configuration/trust prerequisites, sanitized payload fixtures,
  and a minimal reproducible hook-fire canary. Mark unsupported or unknown
  paths explicitly; documentation support alone is not a passed canary.
- [ ] Define a conservative success classifier per tool/platform. Errors,
  denied operations, cancellations, and unknown results must not become
  successes. Distinguish a completed tool call from a changed file and decide
  how no-op patches and multi-file patches count.
- [ ] Decide whether `rate` gains an explicit machine-event mode or shares
  storage with a separate hook endpoint. Keep observed outcome/source distinct
  from subjective quality: do not generate score 5 or use `rate --ok` for
  every successful operation. Preserve issue 181's manual-rating policy.
- [ ] Define session/project attribution and a replay/deduplication key using
  actual available IDs. Avoid duplicates across hooks, manual ratings, and
  shell interception; explicitly record gaps where correlation is unavailable.
- [ ] If viable, specify the smallest integration for verified platforms,
  reusing issue 124's collector where applicable. Preserve unmanaged hooks,
  AGY shim behavior, apply/init command boundaries, and config-driven values.
  Unsupported platforms must remain usable with a documented limitation.
- [ ] Record a go/no-go recommendation and implementation split. Unsupported
  success observation is a valid research result, not permission to infer
  success from an absent error in an unvalidated or incomplete payload.

## 4. Implementation and Verification Guidance

1. Run canaries in disposable files/configuration with isolated telemetry,
   starting with one successful create and edit and one deliberate failure
   on each installed CLI. Include no-op/multi-file patches and interruption
   where supported. Capture bounded hook payloads rather than whole sessions.
2. If implementation proceeds, add fixture tests for classification,
   attribution, duplicate/replayed events, and malformed/unknown payloads.
   Verify automatic observations do not dilute subjective score averages,
   fabricate failure coverage, or reset human/agent heartbeat semantics.
3. Keep hooks quiet and bounded; a telemetry write failure must not invalidate
   the original edit. Prevent recursive recording of the observer itself.
   Store minimal metadata rather than source contents, diffs, or raw results.
4. Test managed installation and repeated apply, cleanup, unmanaged-hook
   preservation, and AGY shim coexistence. Use measured overhead to decide
   whether asynchronous delivery is justified and how completion is flushed.
5. For eventual Go changes, run relevant package tests, `go test ./...`, and
   `make install`; use `scripts/smoke-test.sh` for changed hook installation.
   This filing changes only the tracker; it does not install or enable hooks.
