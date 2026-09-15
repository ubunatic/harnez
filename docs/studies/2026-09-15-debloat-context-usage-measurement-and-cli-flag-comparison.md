<!-- harnez:topic: measured token-context impact of the settings.json debloat feature (issue 316) presets against claude -p "/context", and how that measurement axis compares to the orthogonal --bare and --safe-mode CLI flags -->

# Measuring Debloat's Context Impact, and Why `--bare`/`--safe-mode` Aren't Substitutes

**Scope**: After shipping issue 316 (`harnez apply --debloat` / `status --debloat` /
`revert --debloat`), used `claude -p "/context"` in a clean, isolated test project to
measure the actual token savings of each debloat preset, then compared that against two
built-in `claude` CLI flags (`--bare`, `--safe-mode`) the user asked about as possible
alternatives.

**Accessed**: 2026-09-15
**Updated**: 2026-09-16

## Finding 1 — `claude -p "/context" [--settings '{"permissions":{"deny":[...]}}']` is a real, cheap measurement harness for preset design

Rather than trust the unit tests' JSON-shape assertions as proof the feature does what it
claims, `claude --settings '{"permissions":{"deny":[...]}}' -p "/context"` in a fresh temp
project directory (no `.claude/`, no `CLAUDE.md`) gives an actual measured token delta
without touching any real global or project settings file — `--settings` merges an
additional JSON blob for that one invocation only. This is exactly the kind of live,
real-environment check `docs/AgenticLoop.md` Phase 3 calls for on features whose main
claim is "this actually reduces tokens" rather than "this JSON merge is well-formed" —
the latter was already covered by `internal/claude/debloat_test.go`; the former needed a
real session.

Measured results (clean test project, no memory/CLAUDE.md pollution):

| Configuration | Total tokens | System tools | Deferred tools |
|---|---|---|---|
| baseline (no deny list) | 35.3k | 10.3k | 15.0k |
| `minimal` preset | 35.3k | 10.3k | 10.2k |
| `aggressive` preset | 32.8k | 7.7k | 8.4k |
| `aggressive` + NotebookEdit + Cron* | 32.8k | 7.7k | 6.6k |

## Finding 2 — `minimal` is safe but nearly free; `aggressive` is where the real savings live, and it costs exactly what the external review warned it would

`minimal` (the always-safe, default-on-`--debloat` preset from issue 316's decided
design) only trims ~4.8k off the "deferred tools" line and moves the rounded total by
0%. `aggressive` — which requires the explicit `--debloat-preset aggressive` opt-in
because it denies `AskUserQuestion`, `ScheduleWakeup`, `ReportFindings`, `SendMessage`,
`EnterPlanMode`, `ExitPlanMode` — is where the measurable ~2.5k (~7%) saving actually
is. Layering `NotebookEdit`/`Cron*` on top of `aggressive` buys only ~1.8k more across
four more denied tools, not worth bundling into any default preset.

This validates the 2026-09-11 Codex/Astra external review's core objection from the
ticket (see issue 316) empirically rather than just architecturally: the token cost this
feature can actually remove is concentrated in the interaction/coordination tools, not
the "verified integration-only" ones. There is no free lunch preset — meaningful debloat
is inherently a behavioral tradeoff (fewer clarifying questions, no plan-mode gate, no
structured findings reports, no scheduled wakeups), which is exactly why the decided
design keeps that preset behind an explicit, non-default flag rather than shipping it as
`--debloat`'s default.

Also notable: memory files (13.2k) and skills (3.1k) in this same baseline dwarf
anything either debloat preset touches. If a user wants a bigger context win than
debloat offers, that is a separate, unrelated lever (trimming `CLAUDE.md`/memory
content), not something this ticket's scope should chase.

## Finding 3 — `--bare` and `--safe-mode` measure different, non-composable axes; neither substitutes for or combines meaningfully with debloat

The user asked whether `claude -p "/context" --bare` and `--safe-mode` were relevant
alternatives to check. Measuring both surfaced why they aren't:

- **`--bare`** (2.1k total, "skip hooks, LSP, plugin sync, attribution, auto-memory,
  background prefetches, keychain reads, and CLAUDE.md auto-discovery"): strips the
  entire harness feature surface, not specific tools. Under `--bare`, skills/memory/
  deferred-tools listings are already gone, so `permissions.deny` has nothing left to
  trim — running the same deny lists under `--bare` would show no meaningful additional
  delta. It answers "what does a session cost with harness features off," a different
  question from debloat's "what does a normal session cost with some tools denied."
- **`--safe-mode`** (18.1k total, "disable all customizations — CLAUDE.md, skills,
  plugins, hooks, MCP servers, custom commands and agents, output styles, workflows,
  themes, keybindings — for troubleshooting a broken configuration"): tellingly, its
  "System tools (deferred)" line stayed at **15.0k, identical to baseline** — it does not
  touch `permissions.deny` or tool-schema loading at all. Its token savings come entirely
  from disabling the user's own CLAUDE.md/memory/skills/hooks, i.e. exactly the axis
  debloat is designed to leave untouched. It is also explicitly documented as a
  troubleshooting flag for a broken config, not a token-budget lever, and using it as a
  "debloat" would discard the harnez-managed AgenticLoop docs, memory, custom skills, and
  hooks entirely — a far larger behavioral cost than even the `aggressive` preset.

Net: both flags are useful reference points for the *ceiling* of what disabling harness
features can save, but neither is a knob comparable to, or combinable with, issue 316's
`permissions.deny`-based mechanism. Debloat's contribution is specifically "keep all your
customizations, shed a targeted list of built-in tool definitions" — a narrower, more
surgical claim than either flag makes.

## Finding 4 — the deny list takes effect live, mid-session, not just on next startup

Applied `aggressive` directly to the real `~/.claude/settings.json` (with the user's
explicit go-ahead, apply-then-revert-immediately) to verify the round trip against a real
running session rather than only a temp directory. The denied tools
(`AskUserQuestion`, `SendMessage`, `ScheduleWakeup`, `ReportFindings`,
`EnterPlanMode`/`ExitPlanMode`, plus the `minimal` entries) disappeared from the *current,
already-running* session immediately after `harnez apply --debloat --debloat-preset
aggressive` returned — no restart required. `harnez revert --debloat` restored them
just as immediately, and removed the sidecar `.harnez-debloat.json` record as designed.

This matters for how `aggressive` should be presented to users: toggling it is not a
"next session" decision, it can cut off tools an agent is using mid-task. That is a
reason to keep it opt-in and explicit (as decided), and to warn users who might reach for
it while an agent session is actively running rather than only recommending it be applied
between sessions.

## Finding 5 — `disableBundledSkills` saves 2,120 real input tokens despite `/context` hiding the saving

Issue 348 repeated the `/context` A/B test on Claude Code 2.1.273 in both this repo and a
clean repository outside `~/projects`. The toggle worked functionally in every run: all 12
built-in skills disappeared while all 28 user/project skills remained. `/context` showed
Skills falling from 3.1k to 1.2k, but System tools rising from 7.7k to 9.7k, leaving its
reported total unchanged.

That total is not an API measurement. JSON output confirmed `/context` is local
(`duration_api_ms: 0`, `num_turns: 0`, and zero usage tokens). Two alternating real-prompt
A/B pairs, using `Reply with exactly OK.` from `/tmp` with session persistence disabled,
reported stable server usage:

| Setting | Sonnet input | Auxiliary Haiku input | Combined input |
|---|---:|---:|---:|
| `disableBundledSkills: false` | 37,498 | 897 | 38,395 |
| `disableBundledSkills: true` | 35,378 | 897 | 36,275 |
| Delta | -2,120 | 0 | **-2,120 (-5.5%)** |

Total input here is `input_tokens + cache_creation_input_tokens +
cache_read_input_tokens`. The comparable cache-hit pair returned the same four-token
output, ruling out output variation and cache creation as causes of the input delta.
Therefore `/context` has an accounting defect: it reattributes approximately the removed
skill-listing weight to System tools even though the API request is genuinely smaller.

Decision: make `disableBundledSkills: true` part of both debloat presets. Plain `harnez
apply` remains unchanged; the standalone `--debloat-disable-bundled-skills` flag remains
available for applying only this toggle. If a removed bundled playbook later proves
load-bearing, add a lean harnez-authored replacement for that demonstrated need rather
than restoring the entire recurring catalogue.

## Takeaway for future debloat-adjacent work

When someone proposes a new "reduce context" lever, first check which of these three
non-overlapping levers it actually pulls: (1) built-in tool schema loading
(`permissions.deny`, what issue 316 governs), (2) harness feature surface
(`--bare`-shaped), or (3) user customization content (`--safe-mode`-shaped / CLAUDE.md
and memory trimming). Conflating them — e.g. treating `--safe-mode`'s savings as
evidence for a `permissions.deny` preset choice — would produce a wrong read of where
the tokens actually go, as the "System tools (deferred): 15.0k, unchanged" line above
demonstrates directly.
