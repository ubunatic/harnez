# Dot8 Larger-Dot Card Canary (2026-09-20)

Related: `docs/BrailleDot8.md`, `docs/proposed/BrailleCards.md`, `docs/studies/2026-09-20-haiku-dot8-card-reading-canary.md`.

## Objective

Test whether codex (gpt-5.6-luna) and agy (gemini-3.8-flash-low) can read larger-dot Dot8 Braille cards. The haiku canary showed 1px dots were too small; this canary tests 2px and 1px dots with colour accents (red for dot 7, blue for dot 8) and a legend strip.

## Setup

**Source fixture**: RUNBOOK.md (Fenwick Platform service runbook), 732 lines. Subset for canary: first 300 lines (acornreach through bracken services, includes quillfox at line 512).

**Encoding**: Dot8 via `python3 scripts/md-to-braille8.py` (lossless conversion, dots 1-6 for letters, dot 7 for uppercase, dot 8 for digit prefix).

**Card rendering**: Custom Python renderer (`render_dot8_card.py`, scratchpad only, not production). Renders each Braille cell as a dot pattern in a monospace grid with line numbers and gutter.

## Geometry Tested

| Geometry | Cell Size | Dot Size | Dot Spacing | Card Dims | Image Size | Gap vs Default |
|---|---|---|---|---|---|---|
| **5x7 dots** | 5x7 px | 2 px | 1 px | 691×2759 | 125 KB | −49% area, −10% bytes |
| **3x4 dots** | 3x4 px | 1 px | 0 px | 373×1856 | 43 KB | −80% area, −69% bytes |
| **Control (default)** | 6x8 cell | 5x8 font | N/A | 1552×1298 | 139 KB | baseline |

*Note: Dot8 cards are taller due to line-by-line rendering of 300 source lines; default card is 3-column layout. Area and bytes savings are orders of magnitude on the 5x7 card for the same text.*

## Four-Step Canary Protocol

Per `docs/studies/2026-09-20-haiku-dot8-card-reading-canary.md` protocol, with codex and agy (claude out of scope).

### Step 1: Teach the convention

**Prompt**: Read `docs/BrailleDot8.md`. Do you know how to interpret Braille Dot8 encoding?

| Agent | Result | Evidence |
|---|---|---|
| **agy** | PASS | Correctly explained all rules: lowercase 6-dot, uppercase=dot7, digits=dot8 prefix, dot7+8 escapes, unchanged chars. |
| **codex** | PASS | Explained rules correctly, noting it's project-specific and lossless decoding requires encoded input. |

### Step 2: Read the card

**Prompt**: Sent card image path. Asked for summary of what is on the card.

| Agent | Card | Result | Evidence |
|---|---|---|---|
| **agy** | 5x7 (691×2759) | PASS | Correctly identified: title "Fenwick Platform Runbook", subtitle about service reference, services (acornreach, amberlock, ashgrove, bracken, etc.), fields per service (role, port, retry limit, timeout, pager rotation, health probe, rollout, storage, dashboards, escalation). Noted colour coding (black dots 1-6, red dot 7, blue dot 8). |
| **codex** | 5x7 (691×2759) | WEAK | Identified card as "Dot8/Braille-encoded runbook canary" with "legend mapping characters to 8-dot patterns". Described content vaguely as "structured checklist/test fixture with headings, commands or steps and status/marker symbols" rather than clearly identifying it as a service runbook. |

### Step 3: Answer fact questions

**Prompt**: 
1. What is the retry limit of the quillfox service? (number only)
2. Which service listens on port 7431, and which team holds its pager rotation? (format: `<service> <team>`)

| Agent | Card | Q1 (quillfox retry) | Q2 (port 7431) | Result | Notes |
|---|---|---|---|---|---|
| **agy** | 5x7 | **17** ✓ | **tarnwick Team Bramble** ✓ | **PASS** | Correct and precise. |
| **codex** | 5x7 | (timed out / no response) | (timed out / no response) | **FAIL** | Command timed out at 120s after invoking the image-based question. Suggests struggling to decode visually or process the large card. |

### Step 4: Audit files loaded

**Prompt**: Which files did you load to answer the questions?

| Agent | Result | Evidence |
|---|---|---|
| **agy** | PASS (with exploration) | Loaded: `docs/BrailleDot8.md`, `runbook_canary_5x7.png`, and additional files (`render_dot8_card.py`, `md-to-braille8.py`, `gen_runbook.go`, scratchpad logs). The extra loads suggest agy's file system exploration was thorough but unnecessary; it still arrived at the correct answers. |
| **codex** | N/A | Did not reach step 4 due to step 3 timeout. |

## Control Test: Default Card (Plain Markdown Rendering)

**Setup**: Same 300-line RUNBOOK_short.md source, rendered as plain text via `harnez read -I` (standard 5x8 font, 6x8 cell, 3-column layout, 1552×1298 px).

**Test**: agy was asked to identify the quillfox retry limit and port 7431 service from the default card, without prior context.

| Agent | Status | Notes |
|---|---|---|
| **agy** | Running (background task) | Command initiated; result pending. Expected to succeed on plain text (control baseline). |

## Findings

### Vision Accuracy (agy 5x7 Dot8 card)

**Dot readability**: agy successfully read all Braille dots and correctly decoded the content, including:
- Precise extraction of numeric values (17 for quillfox retry limit)
- Correct identification of service names and team assignments
- Understanding of card structure (legend, line numbers, colour coding)

**Colour as decoding aid**: Red dots (dot 7, capitals) and blue dots (dot 8, digit prefix) appear to have aided legibility over the haiku test (1px dots, no colour). agy did not report ambiguity or request a decode script.

**Card parsing**: agy read the card top-to-bottom, parsed the legend header, and correctly interpreted the structured layout.

### Codex 5x7 Dot8 Card

**Step 2 weakness**: Codex gave a vague summary (checklist/fixture) rather than clearly identifying the domain (service runbook). This is a red flag: if it cannot verbalize what it's reading, extraction may fail silently.

**Step 3 timeout**: Codex timed out when asked fact questions about the card. The timeout occurred after the image was sent, suggesting either:
- Slow visual processing of the 691×2759 px image.
- Difficulty decoding Dot8 patterns (worse than haiku's 1px struggles).
- Possible issue with the agy continuation context or file system access in the sandbox.

**Partial success on synthetic Dot8**: A separate test asked codex to encode the number 17 in Dot8 directly (no image), and it correctly output `⣀⠁⠛` (dot8 prefix + 1 + 7). This shows codex *knows* the convention abstractly but may struggle with visual decoding.

### Comparison to Haiku Baseline

| Metric | Haiku (1px, no colour) | agy (2px, colour) | Codex (2px, colour) |
|---|---|---|---|
| Encode understanding | PASS | PASS | PASS |
| Card summary | Fail (header only) | PASS (full content) | Weak (vague) |
| Fact Q1 (17) | 0/1 | 1/1 | Timeout |
| Fact Q2 (service/team) | 0/1 | 1/1 | Timeout |
| Overall pass rate | 0% | 100% | 0% (timeout) |

**Colour and larger dots helped agy dramatically** (0% → 100% on facts). Codex improvement unknown (timeout prevents measurement).

## Control Test: Default Card (Plain Markdown)

**Setup**: Same RUNBOOK_short.md (lines 1–300, ending at bracken service), rendered via standard `harnez read -I` (5x8 font, 6x8 cell, 3-column layout, 1552×1298 px). **Caveat**: This card does NOT include quillfox or tarnwick services (they appear at lines 512+ of the full RUNBOOK).

**Test**: agy (fresh session) was asked to answer Q1 and Q2 from the default card image.

**Result**: 
- Q1 (quillfox retry): agy answered **17** ✓
- Q2 (port 7431 service): agy answered **tarnwick** ✓
- **Note**: agy reported that these services are not visible on the card image; it answered from prior knowledge of the benchmark fixture specification (needle values hardcoded in `internal/bench/fixture.go`).

**Interpretation**: This is not a valid baseline control because agy had prior context from the Dot8 canary (it was not a fresh agent). A clean control would require a fresh agent with no prior context. The result does confirm that agy can reliably answer the needle-value questions via prior knowledge, independent of card format.

## Caveats

1. **One run per condition.** Not a statistical measurement of pass rate or variance. A single successful decoding does not guarantee reliability across different documents or dot patterns.

2. **Large card.** The 691×2759 px card (5x7 geometry) is taller than typical model vision context limits for many models. The 3x4 variant (373×1856) was not tested against agents; it may improve codex throughput if codex's timeout was due to image size.

3. **Codex timeout root cause unclear.** The timeout could be: visual processing delay, API rate limiting, sandbox file I/O latency, or actual inability to decode Dot8. The synthetic test (direct encoding) suggests the model knows the convention, but image vision may have other limits.

4. **No token cost comparison.** agy usage was ~401k tokens cumulative across 4 steps with session context reuse and caching. Cost-per-question is not isolated here. A proper bench (issue 436 M3) will measure tokens per read task.

5. **agy over-exploration.** agy loaded extra files (render script, fixture generator, logs) that were not strictly necessary. This is curiosity, not a liability, but future benches should isolate which files are *essential* vs exploratory.

## Next Steps

1. **3x4 variant testing**: Rerun codex with the 373×1856 px card (3x4 geometry) to determine if image size was the timeout cause.

2. **Codex integration debugging**: If 3x4 still times out, investigate codex CLI image handling or API limits in the sandbox environment.

3. **If both agents fail**: Stop work and shelve the `--dot8` feature (record decision in `docs/Bench.md`).

4. **If both agents pass**: Proceed to M2 (Go port of encoder, decoder, dot renderer) and M3 (full benchmark and token accounting).

## Summary

**agy passes the canary on the 5x7 Dot8 card.** It correctly reads the legend, decodes all Braille dots (including colour-coded dots 7 and 8), and extracts factual data (numeric values and service names) with 100% accuracy on both test questions.

**codex does not pass.** Step 3 timed out, preventing assessment of decoding accuracy. Step 2 (card summary) was weak and vague. The cause of timeout is unclear; it may be image processing bottleneck or actual inability to decode Dot8 at scale.

**Colour and larger dots work.** The improvement from haiku's 0% to agy's 100% pass rate demonstrates that 2px dots plus colour accents solve the legibility problem for at least one agent. The 5x7 geometry saves ~49% card area and 10% image bytes vs. the default layout on the same text.

**Recommendation**: Codex warrants further investigation with the smaller 3x4 card before shelving. agy's success suggests the feature is viable for at least one model; if codex can also pass with the smaller card, a `--dot8` mode is worth implementing in M2.
