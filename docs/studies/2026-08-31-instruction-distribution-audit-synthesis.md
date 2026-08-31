# Study: Instruction-distribution audit — synthesis (Product Discovery + Technical Advisor)

**Date**: 2026-08-31
**Scope**: Condensed synthesis of two parallel subagent audits — a Product Discovery pass (delivery
mechanism/UX angle) and a Technical Advisor pass (implementation/injection-mechanics angle) — into
the global-vs-project distribution system for agent instructions: how `harnez apply`/`init` write
`CLAUDE.md`/`AGENTS.md`/bundled docs, where that system itself duplicates effort, and what to do
about it. Builds directly on the two clean-session self-audits
([Claude Code](2026-08-31-claude-code-clean-session-system-prompt-audit.md),
[agy](2026-08-31-agy-clean-session-system-prompt-audit.md)), which looked at repetition *inside* a
delivered prompt; this pass looks at the *delivery system* that assembles it.
**Related Issues**: [130](../../issues/130-instruction-distribution-audit-followups.md) (this
audit's follow-up tickets), [128](../../issues/128-per-agent-full-system-prompt-self-audit-for-repetition.md),
[122](../../issues/122-agent-instruction-tool-feedback-protocol.md),
[124](../../issues/124-posttooluse-internal-tool-call-auto-capture.md),
[040](../../issues/040-agent-context-duplication-and-file-read-discipline.md),
[075](../../issues/075-concisemode-promote-doc-to-real-skill.md)
**Status**: Both subagent passes complete; findings condensed below. Concrete fixes tracked in
[130](../../issues/130-instruction-distribution-audit-followups.md), not applied in this study.

---

## 1. Method

Two `general-purpose` subagents ran in parallel, fresh context, read-only (no edits), each with a
self-contained brief pointing at the two existing clean-session studies as required reading:

- **Product Discovery** — mapped what's global vs. project-scoped vs. bundled-by-default today,
  hunted for delivery-mechanism duplication (distinct from in-prompt repetition), checked whether
  the Tool Feedback Protocol section is correctly scoped as *global*, and scouted skill-based
  alternatives to always-on injection.
- **Technical Advisor** — read the actual injection/idempotency/drift-detection code path, evaluated
  concrete technical alternatives for the Tool Feedback Protocol (hook-based auto-capture vs.
  improved prose vs. hybrid), and cross-referenced both prior studies' found repetitions against
  what harnez actually injects, to separate harnez-controlled duplication from purely native-harness
  duplication we can't touch.

Both had access to `config.yaml`, `internal/claude/apply.go`, `internal/markdown/markdown.go`,
`docs/*`, and the existing issue tracker.

## 2. How the distribution system actually works

- **Global** (`harnez apply`, unconditional, every configured target, every session): `config.yaml`
  `agents_md.global.sections` (lines 216–249) — three sections today: "Instructions Hierarchy",
  "Minimal Global Docs", "Tool Feedback Protocol". Written into `~/.claude/CLAUDE.md` **and**
  `~/.prime/agent/AGENTS.md` (`internal/claude/apply.go:596-635`) — both targets, no per-target
  gating.
- **Project-scoped** (`harnez init`): `default: true` docs (Markdown, Git, Canary, Spec,
  AgenticLoop, IssueTracking, Website) install unconditionally on plain `init`; `default: auto`
  docs (Go, Bash, Rust, Zig, Cpp, GoRelease) install based on project-file detection; opt-in-only
  docs (GTK4, Containerfile, ConciseMode) require an explicit `init --docs <name>`.
- **Skills**: `config.yaml:183-214` writes skill files to `~/.gemini/skills`, `~/.codex/skills`,
  `~/.prime/agent/skills` — **there is no Claude-Code-facing skills target at all**. Harnez's own
  skills reach Claude Code only as manually-invoked slash commands (`~/.claude/commands/*.md`), not
  auto-triggered Skills.
- **Mechanics**: each section is wrapped in `<!-- harnez:begin <name> -->`/`<!-- harnez:end -->`
  markers (`internal/markdown/markdown.go:19-22`); `applySection` (109–158) finds-and-replaces or
  appends; idempotency is a literal string-equality check, no hashing. `diff`/`clean` mirror the
  same marker-located logic. **No conditionality mechanism exists for `agents_md.global.sections`**
  — it's a flat, always-applied list. The one place conditional injection *does* exist today is the
  `default: auto|true|false` per-project-file-detection pattern used for language docs
  (`config.yaml:261-407`) — named by the Technical Advisor as reusable prior art, not yet applied to
  global sections.

## 3. Where the delivery system itself duplicates effort

Distinct from the two prior studies' in-prompt findings — this is about harnez's own authored
content being restated across layers with drifted wording, not copy-mechanism duplication (one
canonical source → N project copies, which is expected and fine):

- **"Context Discipline" guidance stated 4 times, independently worded**: `config.yaml:232`
  (global-injected into every project unconditionally), `docs/README.md:7`, `AgenticLoop.md:45`
  Invariant #5, and this repo's own `AGENTS.md:131-134` — four non-identical restatements of the
  same rule rather than one canonical statement referenced three times. This is the *exact*
  "policy-as-prose repeated with no single canonical home" pattern the two clean-session studies
  flagged for native-harness content — but here it's harnez's own content doing it.
- **Issue Priority Schema (P0–P3) hand-copied in full**, not referenced: the complete table +
  severity/priority distinction + metadata header template lives canonically in
  `docs/practices/IssueTracking.md:13-74`, and is copy-mechanism-installed per project as expected
  — but harnez's **own** root `CLAUDE.md` also hand-transcribes the full schema instead of pointing
  at `@docs/IssueTracking.md`, despite that doc being installed locally in this very project. The
  project isn't following its own documented summary-vs-full-doc discipline on itself.
- **`docs/README.md` index accuracy gap**: `DeploymentTransparency.md` is listed as a "Copyable
  doc" but has no corresponding `config.yaml` `languages:` entry — `init --docs` for it would fail
  silently to install anything. The index promises a mechanism that doesn't exist.

## 4. Tool Feedback Protocol: two independent findings converge

Both subagents, working independently, flagged the same section from different angles:

- **Product Discovery (scoping)**: the directive's own invocation template
  (`harnez rate <tool> <score> "<summary>" "<project>/<ticket>"`) presumes the target project uses
  harnez's `issues/NNN-*.md` ticket convention — itself only project-scoped and opt-in. Injecting
  this into *every* project's global config, unconditionally, directly contradicts the adjacent
  "Minimal Global Docs" section in the same file (`config.yaml:236-241`: "global CLAUDE.md... used
  as glue between local and global docs only"). This is project-shaped content sitting in the
  global-only slot.
- **Technical Advisor (mechanism)**: a `PostToolUse` hook (issue 124) **cannot** replace this —
  124's own scope explicitly excludes score ("inherently a judgment call only the agent... can
  make"), and internal tools (Read/Edit/Grep) never pass through a shell the way `harnez exec`'s
  `PreToolUse` rewrite pattern needs (issue 069: "`PostToolUse` cannot rewrite tool output"). A hook
  can at best give a *coverage* signal (call count vs. rated count), never the score itself.
- **Converging recommendation**: this single section needs two independent, non-conflicting fixes —
  (a) rescope it out of unconditional global injection (move to `agents_md.local`/opt-in, or state
  an explicit graceful-degradation for ticket-tracker-less projects), and (b) improve its prose with
  a worked example invocation and an explicit trigger, matching why the git-commit-trailer
  convention is reliably followed (exercised as a literal template every commit) while this
  directive reads as unscoped standing policy. Neither fix substitutes for the other.

## 5. Repetition attribution: harnez-controlled vs. native-harness

The Technical Advisor cross-referenced every repetition the two clean-session studies found against
what `config.yaml`/`internal/claude` actually inject:

- **Zero** of the Claude Code study's repetitions (emoji x3, destructive-op x2, brevity x2) and
  **zero** of the agy study's (anti-polling x3, `file://` links x2) trace back to harnez-injected
  content — both audits ran with zero project docs loaded and still found them; they're entirely
  inside each harness's own built-in system prompt.
- The **only** harnez-controlled instance flagged across both studies is the Tool Feedback Protocol
  non-compliance (§4) — and it's a *missed-instruction* problem, not a *duplication* problem. This
  means 128's "harnez-controlled trims" bucket is currently limited to §3's delivery-system findings
  and §4's protocol fix — the native-harness repetition found in the two prior studies is real but
  out of harnez's control per 128's own stated scope.

## 6. Skill-based alternative distribution

Both subagents converged on the same structural gap: **no `claude_skills_target` write path exists
in `apply.go`/`config.yaml`** — harnez has no way to make its own content auto-triggered for Claude
Code the way built-in skills like `dataviz` are (description-matched, loaded on relevance, not
always in context). Everything currently reaches Claude Code as either always-on global/project
`CLAUDE.md` prose or manually-invoked slash commands.

Candidates named for conversion once that path exists:

1. **`Website.md`** — `default:true` (summarized into every project's `AGENTS.md` even for
   non-website projects), already has a strong trigger condition in its own `docs/README.md` entry,
   already has a `/website` slash command duplicating the same trigger logic manually. Strongest
   candidate: converting it to a real Skill removes one `default:true` doc from every project's
   baseline injection.
2. **`ConciseMode.md`** — issue 075 already promoted it from passive doc to a runtime switch
   (`harnez mode`) that mutates an `AGENTS.md` section; that mechanism predates Skills being
   considered as an option and is heavier (disk-mutating) than a Skill needs to be.
3. **Tool Feedback Protocol itself** — instead of always-on global policy prose, a Skill triggered
   by "after completing an internal tool call in a harnez-tracked project" would give it exactly the
   "worked example, tied to a triggering action" shape §4 identifies as what actually gets
   internalized, loaded only where contextually relevant instead of stamped into every project.

## 7. Consolidated prioritized findings

| # | Finding | Type | Priority |
|---|---|---|---|
| 1 | Tool Feedback Protocol is project-shaped content in the global-only slot, self-contradicting the adjacent Minimal-Global-Docs rule | Misscoping | P1 |
| 2 | Tool Feedback Protocol's prose needs a worked example + explicit trigger to actually get followed (hook auto-capture cannot substitute — score can't be mechanized) | Mechanism | P1 |
| 3 | `docs/README.md` claims `DeploymentTransparency.md` is copyable; no `config.yaml` entry backs it | Index accuracy | P1 |
| 4 | "Context Discipline" guidance independently restated 4x with drifted wording, no canonical source | Delivery duplication | P2 |
| 5 | harnez's own root `CLAUDE.md` hand-transcribes the full P0–P3 schema instead of `@docs/IssueTracking.md`, despite the doc being installed locally | Delivery duplication (self-dogfooding gap) | P2 |
| 6 | "Minimal Global Docs" section could merge into "Instructions Hierarchy" as one bullet (removes a marker pair, no content loss) | Minor consolidation | P3 |
| 7 | No `claude_skills_target` exists — prerequisite for moving any always-on doc to on-demand Skill delivery | Infrastructure gap | P3 |
| 8 | (blocked on #7) Convert `Website.md`, then `ConciseMode.md`, then Tool Feedback Protocol itself to Skills | Distribution redesign | P3 |

None of these were applied here — see [130](../../issues/130-instruction-distribution-audit-followups.md)
for the tickets scoping the actual fixes.

## 8. Toward a repeatable process

This is the second time this line of work has produced a dated, point-in-time snapshot
(the two clean-session studies, and now this synthesis) rather than a lasting artifact that could
be re-run as the project evolves. The method used here is repeatable as-is:

1. Two clean-session subagent introspection passes per target agent (Claude Code, agy, ...),
   per-agent method documented in the two prior studies' §1.
2. A Product-Discovery + Technical-Advisor subagent pair auditing the distribution *system* itself,
   as done in this study.
3. A human-authored (or orchestrator-authored) synthesis condensing both into prioritized findings.

The user raised the idea of eventually packaging this as a self-improvement skill so the whole
assessment can be re-run later with less orchestration overhead. That's tracked as a noted,
not-yet-scoped follow-up in [130](../../issues/130-instruction-distribution-audit-followups.md)
rather than built here — worth doing once the concrete fixes from §7 have landed and the method has
been re-run at least once more, so the skill is built from two real runs' friction, not one.
