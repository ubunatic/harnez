# 441 — Dot8 PNG card reader: decode card content back to text

**Status**: Blocked — Dot8 experiment on hold; see 444
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Reading / Tooling (experimental)
**Related**: 436 (Dot8 cards), `internal/readcard/dot8_render.go`, `internal/readcard/dot8.go`, `docs/BrailleDot8.md`

---

## 1. Problem & Motivation

`harnez read -I --dot8` renders text as 3x4 dot cells. Today nothing reads such a card back. A reader that decodes a Dot8 PNG card into text gives a deterministic ground truth for legibility: if a program can decode the card, the pixels carry the information, and any agent failure is a vision limit rather than a rendering bug. It also gives users a way to recover text from a card.

## 2. /goal

Add a "Dot8 PNG card reader" (for example `harnez read --card-decode <card.png>`, final name open) that reads the main content area of a Dot8 card and prints the result as text. The decoded output should match the source for cards made by the current renderer, verified by a round-trip test: render, decode, compare with `Dot8Decode` of the encoded input.

## 3. Scope

- Input: a PNG produced by `harnez read -I --dot8` (3x4 geometry, 1px dots, columns 0 and 2, accent colours for dot 7 and dot 8, Tom Thumb 3x5 glyphs for non-Braille characters).
- Output: the main content as text, with source line numbers from the gutter when present. Options for raw Braille output versus decoded text are open.
- The header and legend are not required content; the reader may skip or report them.
- Non-Braille characters use the 3x5 font, so decoding them needs glyph matching against the font. Record which characters are ambiguous and report them explicitly rather than guessing.
- Known limits to record: lossy or resampled images, other card styles (`--chrome`, `--gutter`, `--frame`, `--style`) and multi-column layouts need layout detection. Start with cards the renderer itself produced, using layout knowledge shared with the renderer.
- Related to 440: if the header moves into PNG metadata, the reader can take layout parameters from there.

## 4. Notes

- Re-verify against live code and recent commits before starting.
- Keep the reader in Go with no new dependencies (`image/png` from the standard library).

## On hold (2026-09-22)

Parked with the rest of the Dot8 chain. A deterministic decoder is still useless
without a legible card: no agent has passed a clean reading canary yet (haiku failed
outright, codex timed out), and the token benchmark found the braille format's
tokenizer penalty outweighs its compression
(`docs/studies/2026-09-20-dot8-braille-vs-markdown-and-multimodal-context-card-token-benchmarks.md`).
Building the reader before 444's pitch fix would target a geometry that is still
moving. Resume once 444 lands and a clean 3x4 readability canary passes 3/3 on at
least two agents.
