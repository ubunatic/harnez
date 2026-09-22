# Retro: Component System Exploration and Design (2026-09-22)

Scope: issues 489 (component coupling exploration) and 490 (component system design),
plus the follow-ups 491–495. Outcome: docs/HarnezComponents.md §1–8, the MVP package
`internal/components`, a repo-wide Codex hook ownership fix, and recorded user decisions.

## What went well

- **Exploration before design.** 489 was done without reading 490, as asked. The coupling
  analysis (package edges, runtime contracts, use cases) gave 490 a factual base, and the
  recommendation (path A: component selection inside the monolith) fell out of it.
- **MVP in a new package.** `internal/components` wraps `claude.ApplyAllVariant` with
  `Filter` and a removal pass, so the design was testable end to end (full → docs-only →
  full round trip, no drift) without touching `internal/claude`, `cmd/harnez` or
  `config.yaml`.
- **Reviewer subagent paid off.** It found two blocking issues: `codex.Remove` deleted
  user hooks, and the plan and doc claimed an apply step that doesn't exist.
- **Decisions landed as a doc section plus issues.** Open questions became §8.9
  "Decisions" and issues 493/494/495, so the design doc stays evergreen and the work
  stays trackable.

## What went wrong

- **Code before doc.** For 490 I started prototyping inside the main files. The user
  stopped it: a design issue means doc first, and supporting code goes only in new MVP
  packages. The prototype was parked as a patch and the main files restored.
  Lesson: for a "design" or "system" ticket, write the doc first. Code may back the
  doc, but only in new packages, until integration is requested.
- **A latent ownership bug surfaced only when removal was designed.** `codex.Remove`
  had always deleted whole event tables. Nothing noticed, because nothing called it
  with user hooks present. Designing "disabled component removes its entry points"
  forced the ownership question. Now documented in docs/CodexHooks.md.
- **Environment-dependent test.** `TestAgentResumePrintsReplyNotStructDump` failed only
  inside an agent session, because the session env enabled the tip nudge. Now documented
  in docs/Testing.md.
- **Doc path slip.** The design first named the local config `~/.harnez/config.yaml`.
  The real path is `~/.config/harnez/local.yaml`. Check paths against code
  (`usage.LoadLocalConfig`) before writing them into a design.

## Agentic insights

- A removal pass is the real test of an ownership model. Installing is additive and
  forgiving; removing is where "whose entry is this?" must have a precise answer.
- Keep the MVP's mirrored helpers (`SkillTargets` mirrors the unexported
  `skillTargets`) on a short leash: they drift silently. Issue 491 removes them when
  the logic moves into `internal/claude`.
- Pre-existing gofmt drift in ~25 unrelated files makes `gofmt -l` noisy for every
  change. Worth one cleanup commit.
