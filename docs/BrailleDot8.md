---
title: Harnez Braille 8 Convention
---

# Harnez Braille 8 Convention

> **Dot8 card experiment on hold (2026-09-22).** Token benchmarks found Braille
> characters split under the tokenizer so the compression doesn't pay, the
> larger-dot canary's results were invalidated by fixture contamination (only
> the 3x4 geometry saves anything), and no agent has passed a clean reading
> canary yet (haiku failed, codex timed out) while issue 444, an unfixed P1 in
> the renderer, blocks further work. The `--dot8`/`--dot8-colors`/`--dot8-pitch`
> CLI flags are hidden and error out unless `HARNEZ_DOT8=1` is set, and the
> exported `Dot8*` functions in `internal/readcard/dot8.go` are marked
> Deprecated. Resume condition: fix issue 444, then a clean 3x4 readability
> canary passing 3/3 on at least two agents. This encoding convention below
> remains the reference for the encoding itself.

`scripts/md-to-braille8.py` performs a deliberately simple, reversible Unicode
conversion. It is a project convention, not a universal literary- or
computer-Braille table.

## Rules

Unicode Braille dots are arranged as follows:

```text
1 4
2 5
3 6
7 8
```

- Lowercase `a`–`z` use the standard six-dot English Braille alphabet.
- Uppercase letters add dot 7: `a` = `⠁`, `A` = `⡁`.
- Dot 8 (`⢀`) prefixes digits; `a`–`j` mean `1`–`0`:
  `1` = `⢀⠁`, `2` = `⢀⠃`, …, `0` = `⢀⠚`.
- The digit prefix repeats for every digit: `2026` = `⢀⠃⢀⠚⢀⠃⢀⠋`.
- Dot 7 plus dot 8 (`⣀`) escapes literal Braille cells.
  `⣿⣀` therefore encodes as `⣀⣿⣀⣀`.
- Every source character in `U+2800`–`U+28FF` is escaped, including `⣀`.
- All other characters are copied unchanged: Markdown and ASCII punctuation,
  whitespace, emoji, box-art, accented letters, and non-Latin scripts.

There is no word-capitalization mode. Existing unescaped Braille in an input is
ambiguous; lossless decoding requires input produced by this encoder.

## 3px PNG cards

`harnez read -I --dot8 file.md` renders each encoded Braille cell on a fixed
3px horizontal pitch with the same four-pixel height:

```text
1 . 4
2 . 5
3 . 6
7 . 8
```

Each active dot is one pixel at columns 0 and 2. The text font for copied
non-Braille characters is also constrained to the cell pitch, so punctuation,
symbols, and box drawing stay aligned with the Braille stream. The card
renderer may pack several columns, but it does not scale individual cells to
make them larger.

By default, dots 1–6 use the theme text color, dot 7 uses the keyword accent,
and dot 8 uses the type accent. For a diagnostic card, use
`--dot8-colors=red-white` applies to dots 1–6: positions 0, 3, 4, and 7 in
the 2×4 matrix are white; positions 1, 2, 5, and 6 are red. Dot 7 and dot 8
keep their special keyword and type accent colors, respectively. This is a
visual aid, not part of the encoded data, and it applies to the legend and
content cells alike.

The intended visual pattern is:

```text
W R
R W
W R
R W
```

Here `W` is white and `R` is red. The right Braille column is physical pixel
column 2.

## Commands

Encode:

```bash
python3 scripts/md-to-braille8.py input.md
# writes .dot8/input.braille.md beside input.md
python3 scripts/md-to-braille8.py input.md output.braille.md

# Render a native 3px Dot8 card with diagnostic dot colors
harnez read -I --dot8 --dot8-colors=red-white input.md
```

Decode:

```bash
python3 scripts/md-to-braille8.py --reverse .dot8/input.braille.md
# writes input.md beside .dot8/
python3 scripts/md-to-braille8.py --reverse .dot8/input.braille.md restored.md
```

Both normal and reverse conversion perform an automatic lossless round-trip
self-check and refuse to write output if it fails.

Check without writing. Success is silent and exits `0`; failures print only the
affected Markdown section and error, then exit `1`:

```bash
python3 scripts/md-to-braille8.py --check input.md
python3 scripts/md-to-braille8.py --reverse --check .dot8/input.braille.md
```
