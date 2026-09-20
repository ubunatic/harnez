---
title: Harnez Braille 8 Convention
---

# Harnez Braille 8 Convention

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

## Commands

Encode:

```bash
python3 scripts/md-to-braille8.py input.md
# writes .dot8/input.braille.md beside input.md
python3 scripts/md-to-braille8.py input.md output.braille.md
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
