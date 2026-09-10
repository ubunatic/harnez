# 301 — Add /docup testing skill for periodic testing documentation maintenance

**Status**: Open — deferred pending review and assessment
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: `docs/Testing.md`, `AGENTS.md`, `d248997`, `docs/commands/`

---

## 1. Problem & Motivation

Testing guidance is now available in [`docs/Testing.md`](../docs/Testing.md) and
is linked from `AGENTS.md`, but there is no focused workflow for periodically
reviewing whether that guidance still matches the repository. Over time,
testing commands, test layers, canaries, and verification expectations can
change while the documentation remains stale or difficult for an agent to
audit quickly.

Create a future Harnez skill, `/docup testing`, for this maintenance task. The
skill should give an agent a short, repeatable way to inspect the current
testing documentation, identify concrete drift or omissions, and update the
relevant documentation when the review justifies it.

Implementation is intentionally deferred until the issue has been reviewed and
the workflow, scope, and integration points have been assessed.

## 2. Technical Specification / Findings

The command structure should be organized by documentation-maintenance
category:

- `docs/commands/Docup.md` — the parent command and shared workflow;
- `DocupTesting.md` — the first category, focused on quickly reviewing and
  updating testing documentation; and
- later categories such as `DocupArch.md`, `DocupReadme.md`, and other
  narrowly scoped documentation-maintenance workflows as they are justified.

The first category should direct an agent to inspect the relevant testing
guidance, compare it with current repository behavior and conventions, surface
specific discrepancies, and make bounded documentation edits with appropriate
verification. It should account for the existing testing layers described in
`docs/Testing.md`, including package tests, integration-style tests, static
checks, smoke tests, canaries/live checks, and manual or visual verification.

The assessment should resolve how the parent command discovers category
documents, how categories are registered or linked, whether the command is
Claude-specific or shared across supported agents, and how the workflow avoids
turning a periodic documentation review into an unbounded repository audit.

## 3. Implementation & Verification Plan

- Review and assess the proposed `/docup` parent/category command structure
  before implementation.
- Define the shared workflow in `docs/commands/Docup.md` and the first
  category workflow in `DocupTesting.md` only after that assessment.
- Make the testing workflow identify its source documents, relevant code,
  commands, tests, canaries, and recent changes before proposing edits.
- Require concrete findings and bounded updates; preserve accurate existing
  guidance when no change is warranted.
- Verify command discoverability, document links, formatting, and the
  resulting testing guidance with the repository's documentation checks.
- Add later categories only as separate, reviewed extensions of the same
  structure.

No skill, command, or code implementation is included in this ticket's filing
change; this ticket records the future work for review and assessment.

## 4. Assessment / Architecture Notes

Assessment (2026-09-10): **share the workflow and category content, but do not
promise identical slash-command parsing across Codex, AGY, and Claude.** Use one
`docup` skill with an explicitly selected category; this is prompt-level dispatch,
not a portable native subcommand tree. Implementation remains deferred.

- **Existing distribution:** `config.yaml` registers `commands` and `skills`
  separately, commonly pointing both at one `commands/*.md` body.
  `internal/claude/apply.go` (`genCommandContent`, `genSkillContent`,
  `commandTargets`, `skillTargets`) emits Claude commands and skills into
  `~/.claude/commands/docup.md` and `~/.claude/skills/docup/SKILL.md`, and skills
  into configured `~/.codex/skills` and `~/.gemini/skills` roots. Prime also
  receives registered content. The generator adds frontmatter and copies one
  body; it neither expands includes nor copies supporting files. Its schema
  (`internal/claude/config.go: Command`) has no resource manifest or argument
  fields. `/mode <tier>` is an existing quasi-parameterized content precedent,
  not proof of identical argument transport in every harness.
- **Parent and categories:** retain proposed canonical sources
  `docs/commands/Docup.md` and `docs/commands/DocupTesting.md`. That directory
  does not exist today and is absent from `embed.go`'s embed list. Explicitly
  register the parent as the `docup` skill and add the directory to embedding;
  merely adding Markdown files will not distribute them. The parent should
  own an allowlist (`testing -> references/DocupTesting.md`), category summaries,
  argument interpretation, shared limits, and output contract. Category files
  supply evidence entry points and scope, not independent nested `SKILL.md`
  registrations. Read only the selected category, relative to the installed
  skill directory, never relative to the consumer project's cwd. Bare `@docs/`
  links and paths back into the harnez checkout are not portable includes.
- **Packaging recommendation:** extend skill distribution with explicit
  supporting-file mappings, copying the category beside every installed
  `SKILL.md` under `references/`. Cover those files in apply/diff/status/clean,
  idempotency and removal tests. Prefer a skill-only `docup` entry initially:
  Claude exposes skills as slash commands, avoiding two installations with the
  same name and different resource-relative paths. If a legacy command alias is
  required, generate a thin adapter with a resolvable installed skill path.
  `scripts/lint.sh` currently checks only `commands/*.md`; add coverage for the
  new source/resource registration. A smaller alternative is build-time
  composition of parent and category into one skill body, but the generator
  does not do this today and loading all categories undermines bounded reads.
  Keep installation global in `apply`; project document copies belong to
  `init`, following `docs/CLIDesign.md`.
- **Claude:** `/docup testing` can invoke the installed skill. Claude supports
  `$ARGUMENTS`/positional substitution and appends arguments when no placeholder
  receives them. Thus the shared body can interpret the supplied category
  without embedding Claude-only substitutions. Description and native skill
  discovery expose the entry point; optional argument hints or invocation
  controls need generator support, not a second YAML header in the body.
  See [Claude skills](https://code.claude.com/docs/en/skills).
- **Codex:** use explicit `$docup testing` (or select the skill and state
  `category: testing`); treat the category as user-message text, not a guaranteed
  `$ARGUMENTS` expansion or `/docup` custom-command registration. This session
  exposes harnez skills from `~/.codex/skills`; current
  [OpenAI skill documentation](https://learn.chatgpt.com/docs/build-skills)
  instead documents user `~/.agents/skills` and repository `.agents/skills`
  discovery. Verify configured-root discovery on supported Codex versions;
  do not silently migrate roots or install duplicates as part of this skill.
- **AGY:** the installed `agy --help` explicitly documents slash-command and
  skill expansion in print mode, disabled by `--disable-slash-commands`.
  `/docup testing` and `agy -p '/docup testing'` are therefore candidate entry
  points, but help alone does not prove discovery of `~/.gemini/skills`, argument
  preservation, or nested-name syntax. Canary these before claiming support;
  fallback: explicitly request the `docup` skill with `category: testing`.
  Do not equate AGY with Gemini CLI: upstream
  [Gemini custom commands](https://geminicli.com/docs/cli/custom-commands/)
  use TOML, `{{args}}`, and colon namespaces; harnez does not emit that adapter,
  and those docs do not establish AGY behavior.
- **Dispatch/discoverability contract:** advertise supported categories in the
  description and parent help. Missing, unknown, or multiple categories return
  usage and stop without edits; never guess a filename from arbitrary input.
  Accept exactly one allowlisted category from explicit invocation context.
  No shared dependency on shell interpolation, nested slash completion, or
  harness-specific variables. If native argument transport fails its canary,
  retain the shared content with a thin harness adapter or a flat
  `docup-testing` alias supplying a fixed category.
- **Bounded testing workflow:** propose one repository, one category, at most
  10 evidence files and 3 documentation edits per invocation. Start with local
  instructions, `docs/Testing.md`, Makefile/CI recipes, representative tests
  and relevant canary/smoke definitions; inspect at most 10 relevant recent
  commits using bounded searches/reads (AgenticLoop invariant 6). Compare all
  six testing layers already listed above without auditing every test. Stop
  at the cap and report remaining uncertainty. Edit only evidenced drift;
  report a no-op when guidance is accurate. No code fixes, recursive skill or
  sprint invocation, global apply/init, automatic ticket creation, or live
  canary execution just to review documentation. Verify links, formatting and
  documented commands against their definitions; report checks actually run
  separately from checks merely inspected. Follow local commit conventions
  and preserve unrelated changes.
- **Implementation gate:** test installed bundles outside the harnez checkout,
  then canary discovery and category delivery in each actual harness (including
  AGY interactive/print and supported Codex roots). Exercise missing/unknown
  categories, a missing resource, accurate-doc no-op, real drift, and the scope
  cap. These runtime canaries were not run during this assessment; shared
  content is viable, exact cross-harness invocation parity remains unproven.
