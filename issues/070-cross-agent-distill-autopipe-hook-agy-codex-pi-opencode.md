# 070 — Cross-Agent `distill` Auto-Pipe Hook: AGY, Codex, Pi, OpenCode

**Status**: Open — Pi/OpenCode adapters implemented; live liveness verified in [[072-agent-canary-local-llm-pi-opencode]], hook firing still needs a stronger tool-call canary
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Observability & Token Efficiency
**Related**: [[069-pretooluse-hook-optional-autopipe-bash-through-distill]], [[066-native-go-command-output-distillation]], `docs/studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md`

---

## 1. Problem & Motivation

Issue 069 wired `harnez distill` into Claude Code only, via a `PreToolUse` hook in
`~/.claude/settings.json` that rewrites noisy Bash commands to pipe through `harnez distill`
before they run. That mechanism (`hookSpecificOutput.updatedInput`) is Claude-Code-specific.

`harnez` already targets four other agent CLIs for docs/skills distribution (see
`config.yaml`: `prime_agent_target`, `codex_skills_target`, and the sibling npm packages in
`issues/061-containerfile-guidelines-for-fast-incremental-builds.md` — `opencode-ai`,
`@openai/codex`, `@earendil-works/pi-coding-agent`). None of them get the auto-pipe behavior
today; `harnez distill` only helps them if a user invokes it manually.

Targets considered: **AGY** (Google Antigravity / Cortex), **Codex** (OpenAI Codex CLI), **Pi**
(`@earendil-works/pi-coding-agent`), **OpenCode** (`opencode-ai`). See section 3 — RTK prior-art
research narrows this to Pi and OpenCode as the only real hook-based candidates.

## 2. What's Already Known (do not re-research)

From `docs/studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md`:

- **AGY**: has its own `hooks.json` with `PreInvocation`/`PostInvocation`/`PreToolUse`/`PostToolUse`
  events and a different payload shape (`conversationId`, `workspacePaths`, `transcriptPath`,
  `artifactDirectoryPath`, `modelName`, `stepIdx`, `invocationNum`). **Unconfirmed** whether any
  AGY hook event supports rewriting a tool's input before execution (the Claude-Code equivalent of
  `updatedInput`) — this must be verified against AGY's own hook docs before building anything;
  do not assume parity just because event names match.
- **Codex CLI**: "minimal declarative hook support... does not provide a generic `hooks.json`
  lifecycle dispatch." No confirmed mechanism to intercept a shell call before execution. May
  require a different approach entirely — e.g. a wrapper/shim binary on `PATH` ahead of `bash`, or
  giving up on automatic interception and instead improving discoverability of manual
  `harnez distill` usage (e.g. via Codex's `AGENTS.md`-equivalent instructions).

## 3. Prior Art: How RTK (Rust Token Killer) Solves This

Researched via a general-purpose research agent (web search), 2026-08-26. RTK is the same
prior-art tool referenced in issues 065/066 as the inspiration for `harnez distill`'s filters; it
turns out it also already solves the cross-agent auto-pipe problem this ticket is about. Repo:
`github.com/rtk-ai/rtk`; docs at `rtk-ai.app`, notably
`rtk-ai.app/guide/getting-started/supported-agents`.

**Caveat before citing further**: the research agent flagged RTK's reported GitHub star count
(77.5k) as implausibly high for a young Rust CLI and could not fully verify it — treat that number
as unconfirmed/possibly inflated. It does not affect the mechanism findings below, which came from
RTK's own docs.

**General mechanism**: a thin per-agent hook/extension calls into the `rtk` binary itself (`rtk
rewrite <cmd>` or similar) to produce a rewritten command — all the actual logic lives in one
binary, not duplicated per integration. This matches the `internal/distill` + thin-adapter split
already planned below. Also confirmed: RTK treats **exit-code passthrough as a hard requirement**,
**fails open** (runs the original command unchanged if `rtk` itself is missing/erroring, never
blocks the agent), and **skips already-rtk-prefixed or compound commands** to avoid double-piping
— all things issue 069's `harnez distill hook` already does (`set -o pipefail`, the
already-piped guard) or should carry forward here.

**Per-tool findings — this directly resolves section 2's open questions**:

- **Claude Code**: RTK uses the exact same mechanism harnez already built in issue 069 — a
  `PreToolUse` hook rewriting via `updatedInput`, registered in `~/.claude/settings.json`. This is
  independent confirmation that issue 069's approach is the correct/standard one, not a fragile
  guess. (RTK's hook only covers the Bash tool too — same scope limit we already have.)
- **AGY (Antigravity)**: **RTK does NOT do hook-based interception here.** Despite AGY exposing
  `PreToolUse`/`PostToolUse`-named events (per the 2026-08-19 telemetry study), RTK's own docs
  classify AGY as "N/A" for transparent rewrite and instead just writes a plain instructions file
  (`.agents/rules/antigravity-rtk-rules.md`) telling the model to prefer `rtk <cmd>` — prompt-level
  nudging, not enforcement. This resolves section 2's open question: AGY's hook events do not
  support pre-exec input rewriting (or RTK would use it). **Action: do not build a real
  interception hook for AGY; if pursued at all, it should be instruction-file nudging only, and
  should be described to users as such — not as an enforced auto-pipe.**
- **Codex CLI**: Confirmed, matches the existing study's finding. RTK also falls back to
  instruction-file nudging (`AGENTS.md`), not a hook. No real interception mechanism exists.
  **Action: same as AGY — nudging only, or skip.**
- **Pi** (`@earendil-works/pi-coding-agent`): **Real hook mechanism exists.** RTK installs a
  TypeScript extension (`.pi/extensions/rtk.ts` project-scoped, or `~/.pi/agent/extensions/rtk.ts`
  global) hooking Pi's `tool_call` event; Pi auto-discovers extensions from both paths at startup.
  **Action: this is a real, buildable target** — but note the extension is TypeScript, not a
  `harnez`-native Go artifact; would need `harnez` to generate/install a `.ts` file that shells out
  to `harnez distill hook` (or the `harnez` binary directly), similar in spirit to how `commands:`
  /`skills:` already generate files today.
- **OpenCode** (`opencode-ai`): **Real hook mechanism exists.** RTK installs a plugin
  (`~/.config/opencode/plugins/rtk.ts`) hooking OpenCode's `tool.execute.before` event. Same
  TypeScript-generation caveat as Pi applies. (A third-party plugin, `openrtk` by martinstannard,
  npm-installable and wired via `{"plugin": ["openrtk"]}` in `opencode.json`, implements the same
  idea independently — worth a glance for prior art on the plugin shape, not required reading.)

**Net effect on scope**: only **Pi and OpenCode** are real candidates for genuine auto-pipe
interception, matching Claude Code's issue-069 pattern. **AGY and Codex should not get a hook
implementation** — RTK's own maintainers reached the same conclusion after presumably trying, and
building a fake enforcement layer on top of prompt-nudging would misrepresent what the feature
actually does.

## 4. Implementation & Verification Plan

1. Canary-first (per `docs/other/Canary.md`): before writing any `harnez` wiring code, verify
   directly against the actually-installed Pi and OpenCode CLIs that extensions/plugins are
   auto-discovered from the paths RTK's docs describe, and confirm the exact `tool_call` /
   `tool.execute.before` payload shape each passes — RTK's docs are second-hand info, not a
   substitute for a real probe against this machine's installed versions.
2. **Pi and OpenCode only**: add a thin adapter per tool that generates the TypeScript
   extension/plugin file (mirroring how `commands:`/`skills:` already generate files in
   `internal/claude/apply.go`), which shells out to `harnez distill hook` — reuse
   `internal/distill.RewriteBashCommand` as-is; do not duplicate its logic per tool.
3. **AGY and Codex**: do not build a hook. If pursued at all, scope it as documentation-only
   nudging (an `AGENTS.md`/rules-file mention that `harnez distill` exists and is worth piping
   noisy commands through manually) — and be explicit in any user-facing text that this is a
   suggestion, not an enforced auto-pipe, so expectations match issue 065's earlier
   skill-vs-doc lesson about not overclaiming enforcement.
4. Keep all new hook wiring **opt-in**, matching issue 069's `HARNEZ_DISTILL_AUTOPIPE` pattern —
   installing an extension/plugin must never change behavior until a user explicitly enables it.
5. Verify each with a live smoke test against the real installed CLI (not just unit tests of the
   adapter logic), plus `harnez status`.

## 5. Implementation Notes

First buildable phase implemented:

- `config.yaml` now declares `distill_autopipe.pi_extension_target` and
  `distill_autopipe.opencode_plugin_target`.
- `harnez apply` writes managed Pi/OpenCode TypeScript adapters to those targets.
- Both adapters are thin shims that call `harnez distill hook` and consume its
  `hookSpecificOutput.updatedInput.command`; rewrite policy remains in
  `internal/distill.RewriteBashCommand`.
- No AGY or Codex enforced hook was added.
- Unit/integration coverage verifies target expansion, adapter content, apply idempotency, and
  `harnez status` visibility.

Live local Pi/OpenCode liveness was verified in issue 072 against `qwen2.5-0.5b-instruct-q4` through
a dedicated local `lmcoder` proxy. That run did not conclusively prove hook firing: OpenCode loaded
the canary config but the tiny model emitted a non-shell `task` JSON blob for a `git status` prompt,
and Pi returned without an observable tool transcript. A future pass should use a deterministic
tool-call fixture or a more capable small local model to prove the `tool_call` /
`tool.execute.before` rewrite path end-to-end.
