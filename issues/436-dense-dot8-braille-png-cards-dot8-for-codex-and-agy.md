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

- **M1 Canary**: hand-render a larger-dot card of the RUNBOOK fixture with colour accents and a legend. Run the four-step canary protocol from the study on codex and agy. Stop and shelve if both fail.
- **M2 Implement**: Go port of `scripts/md-to-braille8.py` with round-trip tests, then the dot renderer and `--dot8` flag.
- **M3 Bench**: `bench run --read auto --card=--dot8` on codex (luna) and agy, with `--repeat` for stable numbers. Record in `docs/Bench.md`.

## 5. Notes for whoever picks this up

- Re-verify against live code and recent commits first; this ticket may sit for a while.
- An agy smoke run on the default card failed once with 772k input tokens and turns=1. Check whether that is normal for agy before trusting agy numbers.
