# 1004 — TestGoldenFontAssets fails: all five golden font PNGs are stale

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 431, 1002

---

/goal `make test-q1` passes `TestGoldenFontAssets` again, with the cause of the drift named in this
ticket; stop and report if the rendered glyphs changed on purpose and need an owner look.

## 1. Problem & Motivation

`make test-q1` fails on `feat/1002-agy-allowlist` (2026-10-29, harnez 0.2.7+4066), and the same failure
happens with the 1002 changes stashed, so it comes from main:

```
--- FAIL: TestGoldenFontAssets
    golden_test.go:27: 3x5 golden asset is stale; run go run ./scripts/generate-golden-fonts.go
    (same for 5x8, 6x12, 7x13, 8x16)
FAIL	ubunatic.com/harnez/internal/readcard
```

A red quick tier hides new failures, so every agent turn now has to rule this one out by hand.

## 2. Findings

- The test compares the render against `docs/data/golden-font-<size>.png`; all five sizes differ.
- `docs/data/` last changed in 47b8967b (issue 922). The cause (renderer, font spec or Go/image
  change) has not been checked.

## 3. Plan

Find what changed the render. If the change was intended, regenerate with
`go run ./scripts/generate-golden-fonts.go` and look over the new PNGs. If not, fix the renderer.
