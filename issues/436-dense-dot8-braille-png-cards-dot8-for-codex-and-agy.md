# 436 — Dense Dot8 Braille PNG cards (--dot8) for codex and agy

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Reading / Token Efficiency (experimental)
**Related**: `docs/proposed/BrailleCards.md`, `docs/BrailleDot8.md`, `docs/studies/2026-09-20-haiku-dot8-card-reading-canary.md`, `docs/Bench.md`

---

## 1. Problem & Motivation

A Dot8-encoded doc rendered by `harnez read -I` wastes pixels: a Braille cell is 2x4 dots but sits in a 6x8 cell. A haiku canary primed with `docs/BrailleDot8.md` could not read such a card (0 of 3 questions). The project owner reports codex and agy can read Dot8; that is unverified. Claude is out of scope.

## 2. /goal

Decide by measurement whether a `--dot8` card mode of `harnez read -I` is worth keeping: pass rate no worse than the default card on codex (luna) and agy, at a smaller card area or lower image tokens. Otherwise record the result in `docs/Bench.md` and shelve it.

## 3. Scope

- Design and plan live in `docs/proposed/BrailleCards.md`; follow its canary-first plan.
- Bench needs agy first. Done in 13267a0 (`bench run --agent agy`).
- Not a claude feature. Do not make claude read Dot8 cards.

## 4. Milestones

**Rescoped by the project owner (supersedes the canary-first plan):** skip further canaries, drop the 5x7 (2px-dot) path entirely, implement `harnez read -I --dot8` with the 3x4 geometry only, then hand it to the owner for manual testing. Benching (old M3) waits for that manual test.

- ~~M1 Canary~~ dropped; the invalid partial runs are in the study docs.
- **M1 Encoder**: Go port of `scripts/md-to-braille8.py` in `internal/readcard/dot8.go` (encode, decode, lossless round-trip check), tests incl. golden comparison with the Python output on a fixture with digits, capitals, Braille escapes and emoji.
- **M2 Renderer and flag**: 3x4 native dot cells (1px dots, columns 0 and 2, 1px gap column; dots 1-6 in text colour, dot 7 and dot 8 in accent colours), non-Braille characters in the shelved 3x5 Tom Thumb font, legend strip in the header, source line numbers, `--dot8` (encode in-process) and `--dot8=native` (input already encoded) on `harnez read -I`, composing with `--chrome/--gutter/--frame/--meta/--style`. `-L` with `--dot8` returns decoded text. Tests, one golden card, docs updated (`docs/BrailleCards.md` status, read docs), `make install`.
- **M3 Bench** (later, after the owner's manual test): `bench run --read auto --card=--dot8` on codex (luna) and agy, with `--repeat`. Record in `docs/Bench.md`.

## 4a. Milestone log

### M1 delivered (846a8e1), review: NOT yet a valid canary

Study: `docs/studies/2026-09-20-dot8-larger-dot-card-canary.md`. Claimed: agy 2/2 on the 5x7 card, codex timed out at step 3, control unfinished.

Review findings (diff/doc only):

- **Contamination.** agy's step 4 lists `render_dot8_card.py`, `md-to-braille8.py`, `gen_runbook.go` and scratchpad logs as loaded. The card sat in the same directory as the generator and source. The 2/2 may come from the source, not the card. Not a valid pass.
- **Wrong numbers.** 691x2759 vs 1552x1298 is about -5% area, not -49%; 373x1856 is about -66%, not -80%. Bytes (-10%, -69%) are right. The cards also seem to cover different content or layout (1 column vs 3), so the comparison is unfair as written.
- **Fixture text inconsistent.** "first 300 lines ... includes quillfox at line 512".
- **Control missing.** Default-card control never finished; codex has no control at all.
- **Recommendation overstated.** "agy passes the canary" is not supported until the contamination is removed.

### Pre-Work / Required Refinements (before M2)

1. Re-run in a clean directory that holds only the card PNG and `docs/BrailleDot8.md` (copy both there; run the agent with that as cwd). No generator, no source, no logs.
2. Verify from each agent's own step-4 answer that it loaded nothing else; if it did, the run is void.
3. Render the default control card (`harnez read -I`) from the same encoded text and compare area/bytes on identical content. Fix the numbers in the study.
4. Run both geometries (5x7 and 3x4) on both agents. For codex use the bench-style invocation from `internal/bench/agent.go` with a longer timeout; record wall time and whether the timeout was image-related (try a tiny card first).
5. Use `--repeat`-style repetition (3 runs per cell) for the fact questions, and use a fixture answer that the agent cannot guess (check the quillfox retry limit differs from a default).
7. **New (57902b1 control):** agy answered "17"/"tarnwick" from a card that does not contain those services, using the needle values hardcoded in `internal/bench/fixture.go`. So the M1 canary is void: the bench needles are findable in the repo and the 300-line subset lacked quillfox. Do not use the bench fixture for the canary. Generate a small synthetic Dot8 doc with novel random facts (service names, numbers) that appear nowhere in the repo, and put the ground truth outside the agent's cwd.
6. Update the study doc and commit it. Decision rule unchanged: if both agents fail on a clean run, shelve.

## 5. Notes for whoever picks this up

- Re-verify against live code and recent commits first; this ticket may sit for a while.
- An agy smoke run on the default card failed once with 772k input tokens and turns=1. Check whether that is normal for agy before trusting agy numbers.
