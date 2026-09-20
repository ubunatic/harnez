# Haiku Dot8 card reading canary (2026-09-20)

Related: `docs/BrailleDot8.md` (encoding), `docs/proposed/BrailleCards.md` (design),
`docs/studies/2026-09-20-dot8-braille-vs-markdown-and-multimodal-context-card-token-benchmarks.md`.

## Question

Can a claude haiku subagent read a Dot8-encoded Markdown doc from a `harnez read -I`
PNG card, after being taught the convention from `docs/BrailleDot8.md`?

## Setup

- Card: `docs/.dot8/CodexHooks.braille.md` rendered by the current `harnez read -I`
  (5x8 font, 6x8 cell, 3 columns, 149 source lines, 3323 text tokens, one page).
- Ground truth: `docs/CodexHooks.md`, the unencoded source.
- Agent: fresh haiku subagents. No decode script, no source file offered.

## Protocol (one message per step)

1. Fresh haiku. Give only `docs/BrailleDot8.md`, no exploration. Ask whether it knows
   how to handle Braille input.
2. Reuse the agent. Send the card PNG path, ask for a summary.
3. Ask three questions answered in `docs/CodexHooks.md`: the PreToolUse rewrite wrapper
   key and `hookEventName`, the unsupported `permissionDecision` value, and the two
   compaction events.
4. Ask which files it loaded.

## Results

| Step | Outcome |
|---|---|
| 0 (control) | Unprimed haiku given only the PNG: named the file, 149 lines, "Dot8 Braille", hooks topic, colour coding. All of it is on the card header. No body content. It called red lines "comments"; they are list items. |
| 1 | Correct summary of the encoding. Separate check: it encoded "Hi 7" correctly as `⡓⠊ ⢀⠛`. |
| 2 | Header-level summary only (file name, line count, topic). |
| 3 | Answered 0 of 3. Reported the card is not decodable visually and asked for the source or permission to run the decode script. |
| 4 | Loaded two files: `docs/BrailleDot8.md` and the PNG. Never opened `docs/CodexHooks.md` or ran the script. |

## Findings

- Haiku understands the convention from text but cannot read the dots at this cell size.
  Priming does not help; the limit is pixel legibility, not knowledge.
- What it "reads" from the card is the header and layout, which is plain text/colour.
- The test is clean: the agent asked for the source instead of guessing, and the ask was
  declined.

## Caveats

- One agent per condition, one card, one run. Not a pass-rate measurement.
- Only claude haiku was tested by this protocol. See the design doc note on other agents.
- Only the current 1px-dot geometry was tested; larger dots, colour accents and a legend
  are untested.

## Next

Hand-render a canary card with larger dots, accent colours for dots 7 and 8 and a legend
strip, then rerun the same four steps. Bench on the other agents with the existing
`bench run --read auto --card=...` path once a renderer exists.
