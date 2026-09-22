# 440 — PNG card header in metadata instead of image pixels

**Status**: Blocked — Dot8 experiment on hold; see 444
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Reading / Token Efficiency (experimental)
**Related**: 436 (Dot8 cards), 395 (`harnez read -I`), `docs/proposed/BrailleCards.md`, `docs/Bench.md`

---

## 1. Problem & Motivation

Every `harnez read -I` card spends pixels on a header (file name, line count, legend, pointer). Image tokens scale with area, so a header that could travel as text costs image tokens for no gain. PNG can carry it as `tEXt`/`iTXt` chunks instead. Whether codex and agy actually see PNG metadata is unknown; it depends on how each CLI passes the image to the model.

## 2. /goal

Add an opt-in option to `harnez read -I` that writes the header info into PNG metadata chunks and omits the header band from the image. Then measure, on codex (luna) and agy, whether the agent can still identify the file, the line range and (for `--dot8`) the encoding legend. Keep the option only if agents get that info reliably; otherwise document the result and shelve it.

## 2a. Header delivery options

Besides PNG metadata, the header info (same content as today's header band) can travel in two more places. Each is an independent opt-in and can combine with the others, with the image header skipped or kept:

- **Sidecar text file** next to the PNG (for example `card.png.txt`; name and location open). The agent can read it if told the path.
- **Inline in the command output**, appended after the `See <file>.png` note that `harnez read -I` prints. No extra file, and the text reaches the agent through the tool output whatever its CLI does with PNG chunks. This is the most likely to work on codex and agy.

The test compares all four channels (image header, PNG metadata, sidecar file, inline note) on codex (luna) and agy for the same probe questions, and records which ones the agents actually use.

## 3. Scope

- Flag name and shape are open (for example `--meta=png` or `--header=metadata`); pick one that composes with `--chrome/--gutter/--frame/--meta/--style` and `--dot8`.
- The metadata must contain what the header shows today, including the Dot8 legend text and the `docs/BrailleDot8.md` pointer.
- Test with the existing bench harness (`bench run --read auto --card=...`, codex and agy, `--repeat`) plus a direct probe: ask the agent for the file name and line count using only the PNG in a clean cwd.
- Sidecar and inline options carry the same header text and the same Dot8 legend and pointer; the flag design should let one flag select any combination.
- Key uncertainty to record: the agent CLI may strip or never expose PNG chunks to the model. If so, agents only get the pixels, and the header must stay on the image (or be reduced to a minimal strip).
- Not a claude feature; claude is out of scope as in 436.

## 4. Notes

- Re-verify against live code (`internal/readcard`) and recent commits before starting.
- Compare card area and bytes with and without the header on identical content.

## On hold (2026-09-22)

Parked with the rest of the Dot8 chain. This is a pixel-budget optimization for
`--dot8` cards, but the token benchmark showed Dot8 already loses on tokens overall
(braille characters split under the tokenizer, penalty outweighs compression; see
`docs/studies/2026-09-20-dot8-braille-vs-markdown-and-multimodal-context-card-token-benchmarks.md`),
and no agent has passed a clean reading canary yet. Optimizing header placement on a
format that is not yet legible or token-competitive is premature. Resume once 444
lands and a clean 3x4 readability canary passes 3/3 on at least two agents.
