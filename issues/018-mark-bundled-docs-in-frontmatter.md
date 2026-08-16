# 018 — Bundled docs don't self-identify as harnez-managed

**Status:** Open

## Context

Found 2026-08-16 doing an `/evergreen` pass across `uman`, `books`, and
`ubunatic.com` after a working session. Deciding which `docs/*.md` were
safe to hand-edit (project-local architecture/decision notes) versus which
would be silently overwritten on the next `apply` (harnez-bundled)
required finding this line in each project's `AGENTS.md`/`CLAUDE.md`:

```
Docs in `./docs/` are managed by harnez. <!-- harnez:bundled -->
```

(`internal/claude/apply.go:336`, `buildLangConventions`). That marker lives
**only** in the generated `AGENTS.md` "Language Conventions" block — the
bundled doc files themselves (`docs/Canary.md`, `docs/Compliance.md`,
`docs/Make.md`, `docs/Markdown.md`, `docs/Git.md`, `docs/Spec.md`, per-language
docs, …) carry a `title:`/`weight:` front-matter block (see
`installDoc`/`langDocState` in `apply.go`) but nothing that says "this file
is centrally managed, edit the harnez source instead."

Opening `ubunatic.com/docs/Canary.md` directly — the normal way an agent or
a human reads a doc — gives no signal that it differs from
`ubunatic.com/docs/Publishing.md` (project-local, safe to edit) sitting
right next to it in the same directory. The only way to know is to already
know to check `AGENTS.md`, which most doc-reading flows won't do.

This is the same discoverability gap issue 017 hit from a different angle
(a bundled doc's *content* was wrong for the receiving project); here the
problem is that a bundled doc's *managed status* isn't visible from the doc
itself at all.

## Proposal

Add a front-matter key to every doc installed via `installDoc` that marks
it as harnez-managed, e.g.:

```yaml
---
title: Canary-First Development
weight: 20
managed-by: harnez
---
```

Concretely:

1. Add `managed-by: harnez` (or reuse `harnez:bundled` as an
   HTML-comment convention, matching what `AGENTS.md` already uses) to the
   source docs under `docs/` and `docs/lang/` in this repo.
2. `installDoc` already copies bytes verbatim, so no code change is needed
   there — the marker travels for free once it's in the source.
3. Optionally, have `langDocState`'s existing "bundled"/"custom"/"not
   installed" classification surface *why* — right now that logic already
   knows the answer (byte comparison against the bundled source) but only
   exposes it via `harnez status`, not by looking at the file.
4. Downstream `docs/README.md` index files could reflect the marker
   automatically instead of every project hand-annotating which rows are
   bundled (`ubunatic.com/docs/README.md` currently doesn't distinguish at
   all).

## Non-goals

Not proposing to duplicate the full `AGENTS.md` bundled-docs list into every
file's front matter, or to block hand-edits — projects will drift locally
over time and that's expected (`langDocState` already has a "custom"
classification for exactly this). This is purely about a doc announcing its
own provenance to anyone who opens it directly, the same way `weight:`
already announces its place in the sidebar. Checked while filing this:
`ubunatic.com/docs/Canary.md` is currently byte-identical to the
harnez source (`diff` returns clean), so today it's purely a
provenance-signaling gap, not yet a drift-tracking one.
