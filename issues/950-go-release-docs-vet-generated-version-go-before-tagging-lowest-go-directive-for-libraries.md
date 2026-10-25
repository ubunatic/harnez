# 950 — Go release docs: vet generated version.go before tagging, lowest go directive for libraries

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**: goself docs/studies/2026-10-25-goself-bootstrap-to-v0-2-2-story.md (learnings 1, 2)

---

## 1. Problem & Motivation
Two goself releases (v0.1.1, v0.2.1) were pure fixes for gaps the bundled docs do not cover:

1. `harnez release` generated `var Version` in `version.go`, which clashed with the library's
   exported `func Version()`. v0.1.0 was tagged without compiling and had to be retracted.
2. The bootstrap copied the local toolchain (`go 1.26.10`) into `go.mod`. A library with that
   directive forces every consumer's go directive up on `go get`.

## 2. Technical Specification / Findings
- `docs/GoRelease.md` "Version Wiring" does not warn that a package must not already export a
  `Version` symbol, and release does not vet the bumped tree before tagging.
- `docs/Go.md` has no rule for the go directive of libraries.

## 3. Implementation & Verification Plan
1. `docs/GoRelease.md`: add "run `go vet ./...` (or `make check`) on the tree after release writes
   `version.go`, before tagging", plus the `Version` name-clash warning. Consider making
   `harnez release` run the vet step itself.
2. `docs/Go.md`: libraries declare the lowest go directive they need, verified with
   `GOTOOLCHAIN=go1.X.0 go vet ./...`; new-project bootstrap must not copy the local toolchain.
3. Verify: the bundled docs render via `harnez docs`, and a library fixture with `func Version()`
   fails the release check.
