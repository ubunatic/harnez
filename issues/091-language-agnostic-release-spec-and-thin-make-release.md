# Language-agnostic release spec — replace per-project release Make target sprawl

**Status:** Open — proposed vision, needs discussion before implementation
**Severity:** N/A (feature/DX proposal, not a bug)

## Problem

Every sibling project (`spriteview`, `emojig`, `wayreel`, ...) that publishes
releases has independently accreted a large, near-duplicated set of Make
targets: `deps`, `test-minisign`, `release-build`, `release-snapshot`,
`info`, `release-diff`, `release-publish`, `release`, `release-full`, plus
(in emojig) `tag`, `bump-patch`, `bump-minor`, `bump-major`. The
`.goreleaser.yaml` `signs:` block and much of the Makefile skeleton are
copy-pasted verbatim between projects.

This was surfaced concretely while setting up releases for `spriteview`
(a pure-Python project with no compiled build step) by cloning `emojig`'s
(a Zig project) release Makefile and `.goreleaser.yaml`. See
`spriteview/docs/studies/2026-08-28-releasing-a-pure-python-project-with-goreleaser.md`
for the full retrospective, including the language-specific edge cases hit
along the way (GoReleaser's `builds: []` vs `builds: [{skip: true}]`,
`meta: true` archives, Make's `$(wildcard)` caching a stale listing,
Codeberg having no `/releases/latest/download/<file>` alias, etc.).

Two concrete problems came out of that:

1. **Too many Make targets per project**, mostly boilerplate, that has to
   be independently maintained and kept in sync across every project.
2. **No version-lifecycle automation in spriteview** — unlike emojig,
   spriteview has no `tag`/`bump-*` targets, so `make release` assumes a
   git tag already exists. This directly caused a "tag was not made
   against commit X" failure during the spriteview release this session,
   a bug class emojig's `tag` target already avoids.

## Proposed Vision (rough, needs discussion)

Move the release *process* into harnez as a shared, language-agnostic
feature, rather than each project reinventing it in its own Makefile.

- **A version spec, not a version file.** Instead of directly editing a
  language-native version file (`build.zig.zon`, a Python `__version__`,
  a Cargo.toml version, ...), define a `docs/spec`-managed YAML file
  (e.g. `version.yaml`) holding `major`/`minor`/`patch` (and whatever
  else — pre-release tag, build metadata) as the single source of truth,
  consistent with this repo's existing "Spec system" convention
  (`docs/Spec.md`: YAML spec files as source of truth, generated code
  must not duplicate spec values).
- **Harnez owns embedding/generation, per language, not the value.**
  Harnez doesn't care what the version *means* to a given language — it
  only cares about the spec. For languages with native embed support
  (Go's `//go:embed`), harnez wires the spec in directly. For languages
  without embedding, harnez generates a small source file exposing the
  version as a language-appropriate constant (e.g. a generated
  `version.py`, a generated `version.zig`, etc.) from the same spec.
- **A handful of shared Make targets, not a dozen per-project ones.**
  Things like bumping major/minor/patch, tagging, and driving
  GoReleaser become harnez-provided (via a template or a thin
  `harnez release ...` subcommand) rather than hand-copied per project.
- **A build-output convention so harnez can stay language-agnostic.**
  Harnez needs to know how to invoke "produce this project's release
  artifacts" without knowing the language. Proposal: a required,
  standard Make target (name TBD — `make dist`? `make build`? `make
  package`?) that every project implements per its own language/tooling,
  producing artifacts in a conventional location/shape (e.g. `dist/`)
  that harnez's release flow can then pick up, checksum, sign, and
  publish uniformly — for one architecture or many.

### Open questions (deliberately unresolved — needs follow-up research)

- Exact spec shape for `version.yaml` (semver only, or also channel/
  pre-release/build metadata?).
- Which languages need generated-constant support first (Python, Zig
  confirmed via spriteview/emojig/wayreel; others TBD).
- Naming and exact contract of the standard "produce release artifacts"
  Make target — what does it guarantee about output layout, naming,
  multi-arch handling?
- How much of the current per-project `.goreleaser.yaml` stays
  project-owned vs. becomes a harnez-generated/templated file.
- Migration path for `spriteview`, `emojig`, `wayreel` once a design
  exists — none of their current release Makefiles should be touched
  until this is designed and agreed, not reactively patched further in
  the meantime.

## Non-goals for this ticket

This ticket is a **vision/discussion filing**, not an implementation
request. No code changes, no Makefile edits in any sibling project should
be made against this ticket until the design is discussed and refined.

## References

- `spriteview/docs/studies/2026-08-28-releasing-a-pure-python-project-with-goreleaser.md`
  — full retrospective this vision is distilled from, including the
  emojig-vs-spriteview Makefile/`.goreleaser.yaml` diff comparison.
- `docs/Spec.md` — existing harnez convention this proposal extends
  (YAML spec as source of truth, no duplicated values in generated code).
