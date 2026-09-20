# Braille Cards: dot8 text as dense PNG cards

Status: proposed. Nothing here is implemented or benched.
Related: `docs/BrailleDot8.md` (the encoding), `docs/Bench.md` (micro-font path shelved).

## Problem

`docs/.dot8/CodexHooks.braille.md` is the Dot8 encoding of a Markdown doc: letters
become Braille cells, digits get a dot-8 prefix, Markdown and ASCII punctuation stay
as-is. Rendered today by `harnez read -I`, it uses the normal 5x8 font (6x8 cell):

- A Braille cell is 2x4 dots but occupies a 6x8 cell, so most of the cell is empty.
- Digits cost two cells each, so numeric content gets wider than the source.
- The card header and line-number gutter are identical to a text card and give no hint
  that the body is Dot8.

The 3323-token doc fits one 3-column page, but the pixel area is spent on blank space
instead of information.

## Agent support (note)

- **claude cannot handle Dot8 cards today.** A haiku canary, primed with
  `docs/BrailleDot8.md`, could not read the current card and answered 0 of 3 questions
  from it. See `docs/studies/2026-09-20-haiku-dot8-card-reading-canary.md`.
- **codex and agy can**, per the project owner. That has not been verified with the bench
  harness or this canary protocol; re-run it on both before relying on it.
- Consequence: a Dot8 card mode would be agent-specific, so the choice may need to depend
  on the target agent.

## Idea

Give Dot8 text its own card mode that draws each Braille cell natively as a dot block
and packs the grid tighter, while keeping the non-Braille characters legible.

### Cell geometry options

| Option | Braille cell | Punctuation cell | Notes |
|---|---|---|---|
| A. status quo | 6x8 | 6x8 | dots small and sparse, nothing saved |
| B. native dots | 3x4 (1px dots, 1px column gap) | 6x8 font, 2 slots wide | Braille rows cost 4px instead of 8 |
| C. native dots, big | 5x7 (2px dots, 1px gaps) | 6x8 | more robust to resampling, small gain |

Option B halves the line height of pure-Braille lines but breaks the fixed grid: a
line mixing Braille and punctuation needs a 8px band. Simplest workable rule: use one
band height (8px) and pack **two Braille rows per band** is not possible for prose, so
the first implementation is B with fixed 4px-wide slots and a 5x8 font squeezed to
`Font3x5` for the non-Braille characters. That reuses the shelved Tom Thumb font for
punctuation only, where its digit weakness (the luna 12-vs-17 finding) does not apply
because digits are Braille cells.

### Colour as a decoding aid

- Dot 7 (uppercase marker) and dot 8 (digit prefix, escape) drawn in accent colours.
  A model then sees "uppercase" and "digit follows" as colour, not as a 1px difference.
- Dots 1-6 in the normal text colour.
- Markdown structure (`#`, fences, list markers) keeps the existing line colouring from
  `highlightMarkdown` in `internal/readcard/lexer.go`.

### Card header legend

A one-line strip in the header: `dot8 a=⠁ b=⠃ ... z=⠵ A=⡁ dot8=digit prefix` (drawn as
dots). It is a few dozen pixels and lets the agent decode without reading the doc. The
header also carries `docs/BrailleDot8.md` as a text pointer.

## CLI shape

```text
harnez read -I --dot8 docs/CodexHooks.md    # encode in-process, then render as dot8 card
harnez read -I --dot8=native file.braille.md   # already-encoded input, skip the encoder
```

- Port `scripts/md-to-braille8.py` to Go (`internal/readcard/dot8.go`), including the
  lossless round-trip self-check. Go must not depend on Python at read time.
- Style axes stay orthogonal: `--dot8` composes with `--chrome`, `--gutter`, `--frame`,
  `--meta`, `--style`.
- Line numbers refer to the **source** line, so agents can cite `-L` ranges.
- `harnez read --dot8 -L <range>` returns decoded text, so a card is never the only way
  back to the source.

## Risks

- **Vision accuracy.** Reading 1px dots is the hard part. The tight cards already cost
  luna digit accuracy; dot patterns are worse unless colour and legend compensate.
  This is the main thing to measure, not assume.
- **Prompt cost of the convention.** The agent must know Dot8. The legend and a
  one-line instruction in AGENTS.md are part of the cost and must be counted.
- **Lossless only for encoder output.** Unescaped Braille in an input is ambiguous, per
  `docs/BrailleDot8.md`. `--dot8` on a file containing Braille relies on the escape.
- **Token accounting.** Image tokens depend on pixel area, so the win is real only if
  the card gets smaller at equal legibility. Compare bytes and card dimensions first.

## Plan (canary first)

1. Canary: hand-render one 3x4-dot card of the RUNBOOK fixture and ask agy and codex (not claude)
   to answer the `read-one-fact` task from it. Stop if both fail.
2. Implement `dot8.go` encoder and decoder with round-trip tests, plus a golden card.
3. Implement the dot renderer (option B), colour accents and the legend strip.
4. Bench with the existing harness: add read mode variants via
   `bench run --read auto --card=--dot8`, fixtures unchanged, then `--dot8 --style=compact`.
5. Success bar: pass rate no worse than the default card on haiku and luna, with lower
   image tokens or card area. Otherwise record it in `docs/Bench.md` and shelve it.

## Open questions

- Does the input tokenizer cost of an image scale with area or with tile count? Measure
  before trusting area savings.
- Should code fences stay plain text and only prose be Dot8? Code is symbol-heavy and
  gains little from Braille letters.
- Is a per-card legend enough, or should AGENTS.md carry the table?
