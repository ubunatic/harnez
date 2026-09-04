# 230 — Android direct-release scaffold (Makefile template, signing, checksums)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `docs/practices/GoRelease.md`, `docs/other/Website.md`, [[224-initialize-website-rules-and-scaffold-direct-android-releases]] (split from — original ticket's Part B), Smarthome issues 024/025

---

## 1. Problem & Motivation

Split out of [[224]]: harnez's current release conventions work well for Go/Zig archives
published as Codeberg releases, but do not provide a managed baseline for an Android app
distributed directly through an `uman`-managed project page. A prior Smarthome session had to
invent keystore environment variables, APK/AAB targets, signing verification, checksum handling,
and the boundary between building, publishing to Codeberg, and syncing a website — all
project-local, none of it reusable.

224 was split because this scaffold shares no code path with 224's website-doc-install fix; only
the motivating session was shared.

## 2. Findings (carried over from 224)

- Mature sibling release flows separate preflight, tool canary, build, checksum/signing, release
  diff, confirmation, push, and publication. They build and sign before pushing.
- Android needs two independent signatures: Android APK/AAB signing and, optionally, a signed
  checksum manifest for direct downloads. `apksigner verify` must be used; `jarsigner` can report a
  v2-signed APK as unsigned without a failing exit status.
- Direct website publication should remain separate from source/tag publication and require an
  explicit confirmation. `uman website sync` must not happen implicitly during a build.

## 3. Deliverables & Exit Criteria

- [ ] Define a reusable Android direct-release scaffold covering JDK/SDK preflight, version bump,
      tests/lint, APK and AAB builds, external keystore configuration, `apksigner` verification,
      SHA-256 manifests, and optional minisign signatures.
- [ ] Keep `release-build`, source/tag publication, and `uman website sync` as distinct targets;
      provide an interactive orchestrator that states every external mutation before confirmation.
- [ ] Ensure secrets, keystores, APK/AAB outputs, device identifiers, and local configuration are
      ignored and scanned before any first public push.
- [ ] Document how a website download page consumes versioned artifacts without embedding absolute
      subpage paths or depending on a CDN (this is a `docs/other/Website.md` edit, not new
      machinery).
- **Exit criteria**: a fresh Android project initialized by Harnez can adopt a tested,
  non-publishing direct-release workflow without inventing project-local security conventions.

---

## Implementation Plan

Only start after [[224]]'s Part A ships (no dependency in code, but confirms the doc-install
machinery this scaffold's own doc entry will reuse).

1. **New `docs/practices/AndroidRelease.md`**, modeled on `docs/practices/GoRelease.md`'s
   structure (Quick Start / Project Setup Checklist / Releasing & Recovery / Exit Criteria). It
   carries the conventions the Smarthome session had to invent:
   - preflight (JDK + Android SDK versions, `sdkmanager` presence);
   - keystore config strictly via environment (`HARNEZ_ANDROID_KEYSTORE`,
     `..._KEYSTORE_PASS`, `..._KEY_ALIAS`) with the keystore file itself **outside** the repo —
     never a committed path;
   - `apksigner verify --verbose` as the *only* accepted verification (record the finding that
     `jarsigner` reports v2-signed APKs as unsigned with a zero exit status — that trap is the
     reason the doc exists);
   - SHA-256 manifest generation plus optional minisign detached signature, reusing
     GoRelease.md §5.3's language-agnostic signing section rather than restating it;
   - the target boundary: `release-build` (build+sign+checksum, no network), tag/source
     publication, and `uman website sync` are three separate targets; an orchestrator target may
     chain them but must print every external mutation and require an explicit confirmation
     before each.
2. **`config.yaml`** — register the doc: `android-release` entry with
   `source: docs/practices/AndroidRelease.md`, `target: ~/.claude/docs/AndroidRelease.md`,
   `local: ./docs/AndroidRelease.md`, `default: auto`, and a `detectDoc` case on
   `settings.gradle`/`settings.gradle.kts`/`app/build.gradle*`.
3. **`docs/templates/`** — an `android.mk`-style include (mirroring `MakeTargets.mk`) with the
   `release-build`, `verify-signing`, `checksums` targets, plus `.gitignore` lines for
   `*.keystore`, `*.jks`, `*.apk`, `*.aab`, `local.properties`, `*.keystore.properties`.
4. **`docs/other/Website.md`**'s download-page section extended with how a static download page
   references versioned artifacts using relative links only (deliverable 4).
5. Secret-scan gate: document a pre-first-push checklist
   (`git ls-files | grep -Ei 'keystore|\.jks|local.properties'`) in the new doc. Do **not** build
   a scanner into harnez for this ticket.

### Design decisions / tradeoffs

- **Docs + Make template, not Go code.** Harnez has no Android toolchain awareness and should not
  grow one; `harnez release` already delegates the build. Keep the scaffold declarative.
- **No `harnez release` changes.** Android direct-download publication is deliberately outside
  the Codeberg-release path.

### Risks / open questions

- These conventions were derived from one Smarthome session and are unvalidated against a second
  Android project. Writing them as harnez-canonical risks cementing one project's accident.
  Consider marking the doc provisional (`docs/proposed/`) until a second project uses it.
- `apksigner`/`sdkmanager` cannot be exercised in this repo's test suite; this ticket is
  documentation-verified only.

### Scope

**Large** — new practice doc, template, config entry, and no way to test it in this repo.
