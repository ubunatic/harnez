# 986 — Per-project machine-readable glossary and harnez assess --glossary wording check

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 985 (context usage / context window terminology, first consumer), 057 (`harnez assess`), `docs/Spec.md` (spec + JSON schema conventions)

---

/goal Each harnez-managed project can keep a machine-readable glossary (terms, synonyms, forbidden terms, and
more), harnez ships its own glossary starting with the "context" terms, and a harnez check finds wording
anti-patterns in the repo and proposes rewording. Settle the discovery and research questions below first; stop
and report when the scope decision needs the owner. Circuit breaker: on a loop or no progress, clear the goal and
write a failure report here.

## 1. Problem & Motivation

Ambiguous terms lead agents to wrong decisions. In 985, "context size", "context length" and "context used" were
used for both the tokens a session currently uses and the model's maximum, which led to wrong compaction and
restart calls. `agent list` still shows `CONTEXT_USED` next to `CONTEXT_SIZE`. 985 adds the rule as prose in one
ticket, but prose rules spread through tickets and docs are not checkable and do not carry over between projects.

The owner prefers checks in tool output over more instructions. A glossary that tools can read turns wording
rules into a check.

## 2. Proposal (to be confirmed by §3/§4)

- Glossary file per project, for example `.harnez/glossary.yaml`, with a JSON schema under `spec/schemas/`.
  Fields per entry, at least: `term`, `definition`, `synonyms` (allowed), `forbidden` (with reason and suggested
  replacement), `code_names` (identifiers that map to the term), `scope` (paths or file kinds where the rule
  applies), `examples`. Consider `status` (preferred/deprecated) and `see_also`.
- Distributed via `harnez init` the same way as other per-project files; a shared base glossary plus project
  overrides, if §3 confirms that layering is worth it.
- Check command: `harnez assess --glossary` (or a `harnez lint` rule, if that fits better). It scans docs,
  tickets, spec comments, CLI help and output strings for forbidden terms and prints file:line, the term, and a
  proposed rewording. Exit code usable in `make lint`; report-only mode by default.
- Agent-facing: the preferred terms, one line each, reach agents through the existing rules/docs path (no new
  always-loaded block unless it is small).

First harnez glossary entries (from 985):

- **context usage** — tokens the model read on the last call of a turn, incl. cached input and system prompt
  (Claude `/context` term). Code: `ContextTokens`. Forbidden: "context size", "context length", "context used".
- **context window** — the model's maximum input tokens. Code: `ContextWindowSize`. Forbidden: "context size",
  "context length".
- **compaction limit** — the harnez threshold that triggers compaction (`compact_threshold_tokens`); usually
  below the context window.
- **cached re-read** — cached input billed across all calls of a turn; a cost, not a context measure.

## 3. Discovery: is "glossary" the right scope?

Decide whether the glossary should be part of a broader doc-consistency scope, for example:

- a general "doc rules" spec: glossary plus style rules already scattered in `docs/Markdown.md`, ConciseMode,
  `docs/ReviewLearnings.md` (banned phrasings, naming conventions such as PascalCase evergreens);
- a ubiquitous-language model (DDD) that also covers code identifiers, CLI flags and column headers;
- whether existing harnez checks (`harnez lint`, `TestRootDocCopiesMatchSources`, ramp) are the natural home.

Output: a short recommendation with the chosen scope and what stays out.

## 4. Research: how do other agent-focused tools solve this?

Survey and record (with links) how others handle terminology for agents and docs, for example:

- Agent harnesses and rule files: Claude Code (CLAUDE.md, skills), Cursor rules, Windsurf, Aider conventions,
  Codex AGENTS.md, Gemini/Antigravity, Continue, Cline; any glossary or term support.
- Prose linters: Vale (vocabularies, `accept.txt`/`reject.txt`, substitution rules), alex, write-good,
  textlint, proselint; style guides such as Google/Microsoft developer docs.
- Spec or knowledge formats: DDD ubiquitous language, SKOS/term bases, llms.txt.

Output: what to reuse (for example Vale-compatible substitution format) versus build, and whether harnez should
emit or consume an existing format instead of inventing one.

## 5. Verification

- Schema validates the harnez glossary; invalid entries fail `make lint`.
- `harnez assess --glossary` on harnez finds the known `CONTEXT_SIZE` / "context size" uses and proposes the
  replacement; table tests for matching, scope, and code-name exceptions.
- `make test-q1`; `make install`.
