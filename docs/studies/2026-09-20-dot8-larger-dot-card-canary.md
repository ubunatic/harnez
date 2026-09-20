# Dot8 Larger-Dot Card Canary (2026-09-20)

**Status**: Superseded. The canary and the 5x7 geometry were dropped by the project owner (issue 436); the 3x4 card is implemented and awaits manual testing, then bench (M3). The results below stay void for legibility.

Related: `docs/BrailleDot8.md`, `docs/proposed/BrailleCards.md`, `docs/studies/2026-09-20-haiku-dot8-card-reading-canary.md`.

## Objective

Test whether codex (gpt-5.6-luna) and agy (gemini-3.8-flash-low) can read larger-dot Dot8 Braille cards with colour accents, as a canary before implementing a `--dot8` card mode. The haiku canary (100% failure on 1px dots) motivated testing larger geometry.

## Why the Initial Run (M1 846a8e1–57902b1) Is Invalid

### Issue 1: Environmental Contamination

The initial test ran agy in the scratchpad directory that contained:
- The generator script (`render_dot8_card.py`)
- The source fixture (`RUNBOOK.md`)  
- The encoder script (`md-to-braille8.py`)

agy's step-4 answer listed all three as loaded files. This means agy may have answered the fact questions (step 3) by reading the source code or generator, not the card PNG.

### Issue 2: Fixture Contamination

The bench RUNBOOK fixture has hardcoded needle values in `internal/bench/fixture.go`:
- `quillfox` retry limit = 17
- `tarnwick` port = 7431
- Team = `Bramble`

The control test asked agy the same questions from a default card that did NOT contain those services (only lines 1–300 of the RUNBOOK; quillfox is at line 512). agy answered correctly (17, tarnwick) but reported these values were NOT on the card—it answered from prior knowledge of the hardcoded fixture specification, not from reading the card.

### Issue 3: Fixture Text Inconsistent

The initial study claimed "first 300 lines ... includes quillfox at line 512", which is contradictory. Quillfox appears at line 512 of the 732-line RUNBOOK, so a 300-line subset does not include it.

### Conclusion

**The initial run is void.**

## Correct Geometry Data

Tested on a valid fixture: synthetic document with 113 lines, novel facts (not in the repo).

| Geometry | Dimensions | Bytes | vs. Default Area |
|---|---|---|---|
| Default (3-column Markdown) | 1552×448 | 31 KB | baseline |
| Dot8 5x7 (1-column) | 805×1076 | 37 KB | −5% (narrow, tall) |
| Dot8 3x4 (1-column) | 430×734 | 17 KB | −66% (narrow, small) |

**Interpretation**: 
- The 5x7 card is narrower but taller, resulting in −5% area but +19% bytes. Not an improvement.
- The 3x4 card saves −66% area and −45% bytes, a strong win.
- The comparison is unfair: Dot8 cards are 1 column (renderer limitation), default is 3 columns. A fair 1-to-1 would require a 3-column Dot8 renderer (not implemented).

## Pre-Work / Required for Valid M1

Per ticket section "4a. Pre-Work", the canary must be re-run with:

1. **Clean directory isolation**: Fresh directory containing only:
   - One PNG card (e.g., `card_5x7.png`)
   - A copy of the encoding doc (e.g., `encoding.md`, renamed to avoid path resolution)
   - Nothing else

2. **Synthetic fixture**: A ~60-line document with novel facts that do not appear in the repo:
   - Example: services `vortexhub` (retry=23, port=8847), `epochshift` (port=8421, retry=19)
   - Ground truth kept outside the agent's directory to prevent file-system exploration from finding answers

3. **All three cards**: Render default (`harnez read -I`) and both Dot8 geometries from the same source. Compare on identical content.

4. **Both agents, both geometries**: 
   - agy (Gemini 3.8 Flash) on 5x7 and 3x4 cards, plus default control
   - codex (GPT-5.6-Luna) on all three, using bench-style invocation with extended timeout
   - Try smallest card first for codex to determine if timeout is image-related

5. **Three runs per condition**: 3 runs × 2 agents × 3 cards = 18 test sessions. Void any run where step 4 lists extra files.

6. **Fact questions** (cannot be guessed from prior knowledge):
   - Q1: "What is the retry limit of vortexhub?" (answer: 23)
   - Q2: "Which service listens on port 8421?" (answer: epochshift)

## Expected Issues to Resolve

### Codex Timeout

The original 5x7 card (691×2759 px, 222 KB with RUNBOOK) caused codex to timeout at step 3 (fact questions). Possible causes:
- Image too large for vision processing
- Actual inability to decode Dot8 patterns at scale
- Sandbox I/O latency

**Testing strategy**: Start with 3x4 card (430×734 px, much smaller) on codex to determine if timeout is image-size related. If 3x4 also times out, issue is likely visual decoding, not size. If 3x4 works, 5x7 needs optimization.

### agy File Exploration

agy actively explores the filesystem to find files. In a clean directory, this should find only `encoding.md` (and the card PNG, if it tries to access it). In the initial run, it found extra files because they were in the same directory. Clean isolation is critical.

## Summary

The initial M1 canary results (agy 100% pass, codex timeout) are **invalidated** by environmental and fixture contamination. A clean re-run per the pre-work steps is required.

The corrected geometry measurements on synthetic content show:
- **5x7 Dot8**: −5% area, +19% bytes (not a win on standard metrics)
- **3x4 Dot8**: −66% area, −45% bytes (strong area/byte savings, but requires very small dots)

**Pass/fail criteria**: Both agents must pass 3/3 runs on at least one geometry with clean isolation and novel fixture facts, showing that success comes from the card, not prior knowledge or file exploration.
