# 224 — Initialize website rules and scaffold direct Android releases

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `docs/Website.md`, `docs/templates/Makefile`, Smarthome issues 024/025

---

## 1. Problem & Motivation

Harnez exposes a website-building skill that requires every project agent to read
`docs/Website.md` before generating `website/` content. `harnez init` did not copy or bundle that
document into the Smarthome project, even though the canonical document exists in Harnez. This
left the skill unable to proceed and forced manual discovery in a sibling repository.

The same session exposed a second scaffolding gap: Harnez's current release conventions work well
for Go/Zig archives published as Codeberg releases, but do not provide a managed baseline for an
Android app distributed directly through an `uman`-managed project page. The project had to invent
keystore environment variables, APK/AAB targets, signing verification, checksum handling, and the
boundary between building, publishing to Codeberg, and syncing a website.

## 2. Findings

- `docs/Website.md` is a canonical Harnez document but is not currently included in the documented
  copyable `docs/lang`, `docs/practices`, or `docs/other` sets used by `init --docs`.
- The website skill explicitly blocks content generation when the project-local rules file is
  absent, so missing installation is a functional integration failure rather than optional docs.
- Mature sibling release flows separate preflight, tool canary, build, checksum/signing, release
  diff, confirmation, push, and publication. They build and sign before pushing.
- Android needs two independent signatures: Android APK/AAB signing and, optionally, a signed
  checksum manifest for direct downloads. `apksigner verify` must be used; `jarsigner` can report a
  v2-signed APK as unsigned without a failing exit status.
- Direct website publication should remain separate from source/tag publication and require an
  explicit confirmation. `uman website sync` must not happen implicitly during a build.

## 3. Deliverables & Exit Criteria

- [ ] Make `docs/Website.md` an explicitly copyable/bundled document and ensure `harnez init`
      installs it whenever website guidance or a `website/` directory is enabled.
- [ ] Add an init/smoke regression proving a newly initialized website-capable project has the
      exact rules document required by the website skill.
- [ ] Define a reusable Android direct-release scaffold covering JDK/SDK preflight, version bump,
      tests/lint, APK and AAB builds, external keystore configuration, `apksigner` verification,
      SHA-256 manifests, and optional minisign signatures.
- [ ] Keep `release-build`, source/tag publication, and `uman website sync` as distinct targets;
      provide an interactive orchestrator that states every external mutation before confirmation.
- [ ] Ensure secrets, keystores, APK/AAB outputs, device identifiers, and local configuration are
      ignored and scanned before any first public push.
- [ ] Document how a website download page consumes versioned artifacts without embedding absolute
      subpage paths or depending on a CDN.
- **Exit criteria**: a fresh Android project initialized by Harnez can follow the website skill
  without missing docs and can adopt a tested, non-publishing direct-release workflow without
  inventing project-local security conventions.
