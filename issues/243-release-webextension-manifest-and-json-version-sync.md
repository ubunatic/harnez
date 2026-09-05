# 243 — Support WebExtension `manifest.json` and `package.json` Version Sync in `harnez release`

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Release / Tooling
**Related**: [[091-language-agnostic-release-spec-and-thin-make-release]], `internal/release/sync.go`, `internal/release/version.go`, `cmd/harnez/release.go`

---

## 1. Problem & Motivation

`harnez release` provides language-agnostic version spec management (`version.yaml`), semver bumping, and propagation into language files (Go `version.go`, Python `__version__.py`, Rust `Cargo.toml`, Zig `build.zig.zon`).

However, for WebExtensions (e.g. `ffext/linklit`, `ffext/lazypins`) and Node/JavaScript projects, `internal/release/sync.go` does not currently auto-detect or update version strings in `manifest.json` or `package.json`.

Extending `harnez release` to support `manifest.json` and `package.json` enables standard `harnez release` workflows across Firefox extensions and JavaScript tooling.

## 2. Technical Specification

1. **Version Auto-Detection (`AutoDetectCurrentVersion`)**:
   - In `internal/release/sync.go`, check `manifest.json` and `package.json` for `"version": "x.y.z"` when detecting the project's current version.
2. **Version Synchronization (`SyncLanguageFiles`)**:
   - Add updater functions `updateManifestVersion(path, version)` and `updatePackageJSONVersion(path, version)` that update `"version": "..."` in `manifest.json` and `package.json` while preserving formatting/indentation.
3. **Build Command Fallback**:
   - If `build_cmd` is not specified in `version.yaml` and no `.goreleaser.yaml` exists, check if `Makefile` provides a `pack` target or `dist` target and use it as a sensible default.
4. **Subproject Tag Prefix Support**:
   - Allow `version.yaml` to specify `tag_prefix: "linklit-v"` or `tag_template: "{project}-v{version}"` (defaulting to `v` for standard repos) so monorepo subprojects can be tagged independently.

## 3. Verification Plan & Results

1. Unit tests in `internal/release/release_test.go`:
   - [x] Version detection and regex replacement in `manifest.json` (Manifest V3 & V2).
   - [x] Version detection and replacement in `package.json`.
   - [x] Idempotent preservation of surrounding JSON structure.
   - [x] Tag prefix formatting (`tag_prefix: "linklit-v"`, `""`, default `"v"`).
   - [x] Makefile `pack`/`dist` build fallback detection.
   - [x] 2-part semver parsing (e.g. `"1.0"` -> `"1.0.1"`).
2. [x] Run `make test` and `make install` in `harnez`.
3. [x] Dry-run test `harnez release -d ~/git/ffext/linklit --dry-run` confirms it detects `manifest.json` version `1.0`, computes bump `1.0 -> 1.0.1`, falls back to `make pack`, and formats tag cleanly.

