# Pixel Font 5x8 — Non-Trivial Glyphs

Design notes for the hand-tuned glyphs of `RetroPixel5x8` (`Font5x8`, 6x8 cell). Source of truth:
`internal/readcard/spec/glyphs.yaml`, the single list of every glyph with one pixel matrix per font size
(`3x5`, `5x8`, `6x12`, `7x13`, `8x16`; rows are cell-sized strings, `1` = lit). Rendering in
`internal/readcard/font.go`. Golden matrix: `docs/data/golden-font-5x8.png`.

Excluded as trivial: `0-9`, `a-z`, `A-Z`, `! @ # ^ * ( ) { } [ ] - _ + = | < >`.
Trailing empty rows are trimmed in the tables below; each glyph really has 8 rows.

## The grid rules the glyphs follow

- **Cell 6x8, glyph 5x7.** Column 5 is inter-character spacing. Row 7 is empty for most glyphs, so the
  body occupies rows 0-6 and the baseline is row 6. Digits and capitals use all 7 rows.
- **Stem column is 2.** `|`, `!`, `1`, `i`, `l`, `↑`, `↓` and every box-drawing vertical sit on column 2,
  the centre of the 5-wide body. Anything that must connect to a neighbour (box lines, arrows) is anchored there.
- **Narrow marks are 2 px wide on columns 1-2.** Dots, commas, colons and the apostrophe put their right
  column on the stem column. A `.` therefore lines up under the stem of `!`, `i` and `1`, and a 2x2 dot
  survives at 1x scale where a single pixel would read as noise.
- **One idea per glyph, no anti-aliasing.** With about 35 pixels per glyph, each glyph keeps only the features that
  distinguish it from its nearest confusable neighbour. The tables name that neighbour.
- **Diagonals are exact 45 degree steps** (1 px per row), so `/`, `×`, `%` and the arrow heads look like straight strokes.

## Punctuation

Two-pixel-wide marks that are vertically centred in the 7-row body (rows 0-6) or sit on the baseline.

| Glyph | Code | Pixels (6 cols) | Why it works at 5x8 |
|---|---|---|---|
| `.` | U+002E | `······`<br>`······`<br>`······`<br>`······`<br>`······`<br>`·##···`<br>`·##···` | 2x2 block on rows 5-6, so it rests on the baseline. Two pixels tall makes it visible next to a 1 px stem; its right column matches the stem column. |
| `,` | U+002C | `······`<br>`······`<br>`······`<br>`······`<br>`·##···`<br>`··#···`<br>`·#····` | A tick (`.##/..#/.#.`) hanging from the baseline: the same shape as `'`, moved down. It is the only comma shape that stays distinct from `.` in 3 rows. |
| `:` | U+003A | `······`<br>`·##···`<br>`·##···`<br>`······`<br>`·##···`<br>`·##···` | Two 2x2 dots on rows 1-2 and 4-5, with one blank row above and below. Centred in the body band so it stays clear of `.` (rows 5-6) when they neighbour each other. |
| `;` | U+003B | `······`<br>`·##···`<br>`·##···`<br>`······`<br>`·##···`<br>`··#···`<br>`·#····` | The upper colon dot (rows 1-2) over a comma tick (rows 4-6). Because it is built from the `:` dot and the `,` tick, it reads as a mix of the two and cannot be mistaken for either. |
| `'` | U+0027 | `·##···`<br>`··#···`<br>`·#····` | The comma tick on rows 0-2, i.e. `,` mirrored to the top. The slant (`.##/..#/.#.`) says "apostrophe or comma" without spending pixels on a curve. |
| `"` | U+0022 | `·#·#··`<br>`·#·#··`<br>`·#·#··` | Two vertical 3-px bars (columns 1 and 3) on rows 0-2. Bars stay legible where two slanted ticks would merge into a smear; the gap column keeps them as a pair. |
| `/` | U+002F | `····#·`<br>`···#··`<br>`··#···`<br>`·#····`<br>`#·····` | A clean 5-row diagonal from (4,0) to (0,4). It stops at row 4 so it does not read as a descender or collide with `\` or `1` in mixed text. |
| `?` | U+003F | `·###··`<br>`#···#·`<br>`····#·`<br>`···#··`<br>`··#···`<br>`······`<br>`··#···` | Hook on rows 0-4 (`.###.`, `#...#`, then a diagonal back to the stem), a blank row 5, and the dot on row 6. The gap is what makes it `?` and not an odd `2` or `7`. |
| `\` | U+005C | `#·····`<br>`·#····`<br>`··#···`<br>`···#··`<br>`····#·` | The exact mirror of `/`, a 5-row diagonal from (0,0) to (4,4), so the pair reads as a matched set. |
| `` ` `` | U+0060 | `·#····`<br>`··#···`<br>`···#··` | A 3-px diagonal in the same direction as `\`, on rows 0-2 and centred on the stem column. It is a short accent, distinct from the `'` tick. |
| `~` | U+007E | `······`<br>`······`<br>`·##·#·`<br>`#·##··`| A two-row wave (`.##.#` over `#.##.`) centred on row 3 with the other operators. Two rows are the minimum for a visible wave. |

## Symbols and currency

Glyphs that would need curves or many strokes. Each is reduced to its identifying skeleton.

| Glyph | Code | Pixels (6 cols) | Why it works at 5x8 |
|---|---|---|---|
| `$` | U+0024 | `··#···`<br>`·####·`<br>`#·#···`<br>`·###··`<br>`··#·#·`<br>`####··`<br>`··#···` | An `S` built from angular strokes, plus stem pixels above (row 0) and below (row 6). The two extra pixels are what separates it from `S` and `5`. |
| `%` | U+0025 | `##····`<br>`##··#·`<br>`···#··`<br>`··#···`<br>`·#····`<br>`#··##·`<br>`···##·` | Two 2x2 blocks at opposite corners (columns 0-1 top, 3-4 bottom) joined by a 1 px diagonal. Small circles cannot be drawn in 5 px, but a 2x2 block plus a slash reads unambiguously as "percent". The pair is point-symmetric. This is the fix for issue 430. |
| `&` | U+0026 | `·##···`<br>`#··#··`<br>`#·#···`<br>`·#····`<br>`#·#·#·`<br>`#··#··`<br>`·##·#·` | A stacked loop, a diagonal crossover and a foot on the right (`.##`, `#..#`, `#.#`, `.#.`, `#.#.#`, `#..#`, `.##.#`). A lone loop would look like `8` or `B`, so it uses the crossing strokes and the trailing tail. |

## Superscripts

The 5-row digit bodies of the normal `0-9`, on rows 0-4 and top-aligned. Rows 5-7 stay empty, so they render as small raised digits without a separate scale.

| Glyph | Code | Pixels (6 cols) | Why it works at 5x8 |
|---|---|---|---|
| `⁰` | U+2070 | `·###··`<br>`·#·#··`<br>`·#·#··`<br>`·#·#··`<br>`·###··` | Ring `.###./.#.#./.#.#./.#.#./.###.`, three columns wide. Narrower than `0`, so it cannot be mistaken for the `°` sign. |
| `¹` | U+00B9 | `··#···`<br>`·##···`<br>`··#···`<br>`··#···`<br>`·###··` | Flag-and-foot `1`. The foot serif keeps it distinct from `I` and `l`. |
| `²` | U+00B2 | `·###··`<br>`····#·`<br>`··##··`<br>`·#····`<br>`#####·` | Hook, diagonal, then a full-width base bar. The base bar is the identifying feature. |
| `³` | U+00B3 | `·###··`<br>`····#·`<br>`··##··`<br>`····#·`<br>`·###··` | Two right-facing bowls with the middle pinched to the left (`..##.`). |
| `⁴` | U+2074 | `··##··`<br>`·#·#··`<br>`#··#··`<br>`#####·`<br>`···#··` | Open diagonal, crossbar on row 3 and a stem that extends through it. |
| `⁵` | U+2075 | `#####·`<br>`#·····`<br>`####··`<br>`····#·`<br>`####··` | Top bar, left drop, bowl. Same skeleton as `5`. |
| `⁶` | U+2076 | `·###··`<br>`#·····`<br>`####··`<br>`#···#·`<br>`·###··` | Top curve plus a bowl on the bottom. The open upper-left is what tells it from `⁸`. |
| `⁷` | U+2077 | `#####·`<br>`····#·`<br>`···#··`<br>`··#···`<br>`·#····` | Top bar plus a 45 degree diagonal. |
| `⁸` | U+2078 | `·###··`<br>`#···#·`<br>`·###··`<br>`#···#·`<br>`·###··` | Two stacked rings sharing the middle row. The narrowest way to show two loops. |
| `⁹` | U+2079 | `·###··`<br>`#···#·`<br>`·####·`<br>`····#·`<br>`·###··` | A `⁶` rotated by 180 degrees: bowl on top, tail to the bottom-left. |

## Maths and units

Mostly 5-wide, centred on column 2. The vertical axis of symmetry is kept so operators look balanced next to digits.

| Glyph | Code | Pixels (6 cols) | Why it works at 5x8 |
|---|---|---|---|
| `°` | U+00B0 | `··#···`<br>`·#·#··`<br>`··#···` | A 3x3 diamond (`..#./.#.#/..#.`) on rows 0-2. The diamond is the smallest ring that still has a hole. Every font size in `glyphs.yaml` uses a diamond. |
| `±` | U+00B1 | `··#···`<br>`··#···`<br>`#####·`<br>`··#···`<br>`··#···`<br>`······`<br>`#####·` | A plus (rows 0-4) with a separated bar on row 6. The blank row 5 keeps the two parts distinguishable from a `+` over an `=`. |
| `×` | U+00D7 | `······`<br>`#···#·`<br>`·#·#··`<br>`··#···`<br>`·#·#··`<br>`#···#·` | A full-width X across rows 1-5, centred on row 3 like the bars of `+`, `-` and `=`. It is smaller than a capital `X` (rows 0-6), so it reads as an operator. |
| `÷` | U+00F7 | `······`<br>`··#···`<br>`······`<br>`#####·`<br>`······`<br>`··#···` | A 5-wide bar on row 3 with a dot above (row 1) and below (row 5). Dots share the stem column, and the symmetric spacing stops it from looking like `:` or `=`. |
| `≤` | U+2264 | `···#··`<br>`··#···`<br>`·#····`<br>`··#···`<br>`···#··`<br>`······`<br>`·###··` | A 5-row chevron opening to the right on rows 0-4, then a gap row and a short 3 px bar on row 6. The bar is 3 px, not 5, so it fits under the chevron point without merging. |
| `≥` | U+2265 | `·#····`<br>`··#···`<br>`···#··`<br>`··#···`<br>`·#····`<br>`······`<br>`·###··` | The mirror of `≤`. Every diagonal has an identical slope to `<` and `>`, so the chevron reads as `<` and `>` extended. |

## Arrows and indicators

Anchored on the stem column so they align with box-drawing lines.

| Glyph | Code | Pixels (6 cols) | Why it works at 5x8 |
|---|---|---|---|
| `↑` | U+2191 | `··#···`<br>`·###··`<br>`#·#·#·`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···` | Chevron head (`..#.`, `.###`, `#.#.#`) over a 4-row stem. The head widens over 3 rows, so its slope is steeper than a plain `^`, which keeps it visually heavier than the caret. |
| `↓` | U+2193 | `··#···`<br>`··#···`<br>`··#···`<br>`··#···`<br>`#·#·#·`<br>`·###··`<br>`··#···` | `↑` flipped vertically. Head on rows 4-6 with the tip on the baseline row. |
| `↔` | U+2194 | `······`<br>`······`<br>`·#·#··`<br>`#####·`<br>`·#·#··` | A 5-wide shaft on row 3 with an arrowhead pair on row 2 and row 4 at columns 1 and 3. Only 3 rows are used because one row of head above and below the shaft is the smallest recognisable head. |
| `→` | U+2192 | `······`<br>`··#···`<br>`···#··`<br>`#####·`<br>`···#··`<br>`··#···` | Shaft on row 3 (5 wide), head as a 3-row wedge (rows 1-5, columns 2-3-2). Head slope 45 degrees. It uses 5 rows, so it stands out from the `↔` head. |
| `←` | U+2190 | `······`<br>`··#···`<br>`·#····`<br>`#####·`<br>`·#····`<br>`··#···` | `→` mirrored: head on the left, shaft on row 3. |
| `↳` | U+21B3 | `#·····`<br>`#·····`<br>`#·····`<br>`#·#···`<br>`#··#··`<br>`#####·`<br>`···#··`<br>`··#···` | Stem on column 0 (rows 0-2), a diagonal bend on rows 3-4, a bar on row 5, and a tip that continues to rows 6-7. It uses row 7, unlike almost everything else, and the shape does not clearly read as a return arrow. Worth a second look. |
| `✓` | U+2713 | `······`<br>`····#·`<br>`···#··`<br>`#·#···`<br>`·#····` | A short left leg (`#.#` on row 3, tip on row 4) and a long right leg rising to (4,1). Two diagonals meeting at a point are enough for a tick. |
| `✗` | U+2717 | `#···#·`<br>`·#·#··`<br>`··#···`<br>`··#···`<br>`·#·#··`<br>`#···#·` | A full-height X with a doubled centre (rows 2 and 3), which makes it a taller, heavier cousin of `×`. Height 6 versus 5 is the difference from `×`. |
| `…` | U+2026 | `······`<br>`······`<br>`······`<br>`······`<br>`······`<br>`······`<br>`#·#·#·` | Three pixels on row 6 with 1 px gaps (`#.#.#`). The three dots fit into the 5 px body width, so it fills the cell exactly like a real ellipsis. |
| `•` | U+2022 | `······`<br>`······`<br>`··##··`<br>`·####·`<br>`·####·`<br>`··##··` | A 4x4 rounded blob on rows 2-5 (`.##.`, `####`, `####`, `.##.`). Corner pixels are cut to give a round look, while the size keeps it well above `·` or `.`. |
| `⚠` | U+26A0 | `··#···`<br>`·###··`<br>`·#·#··`<br>`##·##·`<br>`#####·`<br>`##·##·`<br>`#####·` | A filled triangle with a knocked-out exclamation (rows 3 and 5 have a `..` gap). Pixel outline tapers 1 to 5 columns wide, and the gaps form the `!` inside. |
| `✦` | U+2726 | `······`<br>`··#···`<br>`·###··`<br>`#####·`<br>`·###··`<br>`··#···` | A four-point star: diamond in rows 1-5 with the widest row in the middle (`#####`). A plus shape with filled quadrants. |

## Dotted lines and braille



| Glyph | Code | Pixels (6 cols) | Why it works at 5x8 |
|---|---|---|---|
| `┈` | U+2508 | `······`<br>`······`<br>`······`<br>`##·#·#` | Row 3 pattern `##.#.#`, running edge to edge (columns 0-5). Because it fills the full cell width, adjacent cells join into a dashed line with a regular period rather than doubled gaps. |
| `┄` | U+2504 | `······`<br>`······`<br>`······`<br>`#·#·#·` | Row 3 pattern `#.#.#.`, a plain alternation. The two dashed lines differ in phase, and the pair gives a short-dash and a long-dash variant. |
| `⣿` | U+28FF | `·#··#·`<br>`······`<br>`·#··#·`<br>`······`<br>`·#··#·`<br>`······`<br>`·#··#·` | Dots at columns 1 and 4 on rows 0, 2, 4, 6: the 2x4 braille matrix, with 3 px horizontal pitch and 2 px vertical pitch. This glyph is drawn procedurally by `drawBrailleRune` (all `U+2800-28FF`) before the glyph table is consulted, so the table entry is currently unused. The procedural output matches it. The `height == 8` special case in `font.go` exists to hit these exact rows. |

## Box drawing

Vertical strokes on column 2 and horizontal strokes on row 3 in every glyph, so neighbours always join. The horizontal runs go to the cell edge (column 5) on the side that continues, and stop at column 2 on the side that does not.

| Glyph | Code | Pixels (6 cols) | Why it works at 5x8 |
|---|---|---|---|
| `─` | U+2500 | `······`<br>`······`<br>`······`<br>`######` | Full-width bar on row 3. |
| `│` | U+2502 | `··#···`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···` | Full-height stem on column 2, rows 0-7. Runs through the blank row 7, so vertical lines connect between text rows. |
| `┌` | U+250C | `······`<br>`······`<br>`······`<br>`··####`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···` | Right arm from the stem (columns 2-5) on row 3, stem going down (rows 4-7). |
| `┐` | U+2510 | `······`<br>`······`<br>`······`<br>`###···`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···` | Left arm (columns 0-2) on row 3, stem going down. |
| `└` | U+2514 | `··#···`<br>`··#···`<br>`··#···`<br>`··####` | Stem coming from above (rows 0-2), right arm on row 3. |
| `┘` | U+2518 | `··#···`<br>`··#···`<br>`··#···`<br>`###···` | Stem from above, left arm on row 3. |
| `├` | U+251C | `··#···`<br>`··#···`<br>`··#···`<br>`··####`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···` | Full stem plus right arm. |
| `┤` | U+2524 | `··#···`<br>`··#···`<br>`··#···`<br>`###···`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···` | Full stem plus left arm. |
| `┬` | U+252C | `······`<br>`······`<br>`······`<br>`######`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···` | Full-width bar on row 3, stem going down. |
| `┴` | U+2534 | `··#···`<br>`··#···`<br>`··#···`<br>`######` | Full-width bar on row 3, stem coming from above. |
| `┼` | U+253C | `··#···`<br>`··#···`<br>`··#···`<br>`######`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···` | Full stem and full-width bar. |
| `╭` | U+256D | `······`<br>`······`<br>`······`<br>`··####`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···` | The same pixels as `┌`. At 1 px stroke width, a rounded corner cannot be told from a square one, so the rounded glyphs reuse the square geometry and keep the joins exact. |
| `╮` | U+256E | `······`<br>`······`<br>`······`<br>`###···`<br>`··#···`<br>`··#···`<br>`··#···`<br>`··#···` | Same pixels as `┐`. |
| `╰` | U+2570 | `··#···`<br>`··#···`<br>`··#···`<br>`··####` | Same pixels as `└`. |
| `╯` | U+256F | `··#···`<br>`··#···`<br>`··#···`<br>`###···` | Same pixels as `┘`. |

## Observations for future tuning

- **Rounded corners are duplicates.** `╭ ╮ ╰ ╯` are pixel-identical to `┌ ┐ └ ┘`. That is a deliberate trade-off at this size, but it means rounded-vs-square information is lost.
- **`↳` looks irregular** (see its row above). Compare against the import in `docs/data/golden-font-5x8-import.png` before treating it as final.
- **Braille is procedural.** All `U+2800-28FF` cells are drawn by `drawBrailleRune`, so `⣿` has no entry in `glyphs.yaml`. The `height == 8` special case in `font.go` hits the rows `0, 2, 4, 6`.
- **Other sizes are upstream fonts** (see `third_party/fonts/README.md`); only the default 5x8 is hand-tuned. Glyphs missing upstream render as `?`: 6x12 lacks `→ ← ✓ ✗`, 7x13 lacks `✓ ✗`.
