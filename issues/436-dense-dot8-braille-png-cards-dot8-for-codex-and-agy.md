# 436 — Dense Dot8 Braille PNG cards (--dot8) for codex and agy

**Status**: Blocked — Dot8 experiment on hold; see 444
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

### M1 encoder delivered (db3cd96), M2 renderer partly delivered (inside 5359046)

Review (diff, tests, sample card `docs/CodexHooks.md`): encoder and its tests look fine; `go test ./internal/readcard ./cmd/harnez` passes. M2 is incomplete.

### Pre-Work / Required Refinements for M2

1. **Bare `--dot8` is broken.** `harnez read -I --dot8 file.md` takes the path as the flag value and fails with "invalid --dot8 value". Make bare `--dot8` mean encode (`NoOptDefVal`, like `--multi`), keep `--dot8=native`, reject other values. Test all three plus the invalid case.
2. **Canvas is not sized to content.** The sample card is 1234x1560 (1.92M px), but the content fills only about 380x900 px in the top-left; the rest is empty. The default card of the same file is 1552x628 (0.97M px). Size the canvas to the content, use the same column layout as the default card (`--columns`, `--max-dim`), and check the result is smaller than the default card; if not, report the numbers honestly.
3. **Legend strip missing** (required by M2). Add the one-line dot legend and the `docs/BrailleDot8.md` pointer in the header.
4. **No tests for `dot8_render.go`.** Add pixel-level assertions (dot positions, accent colours of dot 7 and 8), a composition test with `--style=compact`, and the golden card. Add flag tests in `cmd/harnez` (bare, native, invalid, `-L` returns decoded text, source line numbers in the gutter).
5. **Docs.** Update `docs/proposed/BrailleCards.md` status (3x4 implemented, 5x7 dropped) and the doc listing `harnez read` flags. Run `make install`.
6. **Commit hygiene.** The renderer landed inside another agent's commit ("docs(issues): add ticket 438") via a staging race. Do not rewrite history. From now on: `git add <explicit paths>`, check `git diff --cached --stat`, and `git commit -- <paths>` so only your files go in.
7. Decoding in text mode currently decodes lines that the same command encoded; simplify so `--dot8` without `-I` returns the plain source lines unchanged (and `native` decodes).

### M2 delivered (36cd9ff, f808d55), review: content is clipped

Flags, legend, tests and `make install` are in and `go test ./internal/readcard ./cmd/harnez` passes. But the sample card of `docs/CodexHooks.md` (1234x368) shows only source lines 1-52 in the left column and is cut at the bottom edge; the other two columns are blank, although `harnez read` reports "3 col, 149 lines". The claimed "47% area" is therefore invalid: content is lost, not compressed.

### Pre-Work / Required Refinements (M2b)

1. Reproduce first: a test that renders `docs/CodexHooks.md`-like input (149 lines) with `--dot8` and asserts that the last source line number appears in the image (for example by checking that ink exists near the bottom of each column and that all three columns contain content), and that no line is clipped. It must fail on the current code.
2. Fix the column layout and canvas height so all source lines are drawn across the columns like the default card.
3. Re-measure against the default card of the same file (1552x628, 54 KB) and report honest dimensions and bytes.
4. Existing tests are green despite the clipping, so tighten the render tests (item 1) rather than only adding new ones.
5. Commit with `git add <paths>` and `git commit -- <paths>`; `make install`.

### M2b delivered (7f84585): ready for owner manual test

Clipping fixed (regression test `TestDot8RenderAllColumnsContainUnclippedContent`). `docs/CodexHooks.md`: Dot8 card 1234x368, 20.9 KB, all 149 lines across 3 columns; default card 1552x628, 54 KB (about -53% area, -61% bytes). Legibility to agents is unmeasured; that is M3, after the owner's manual test. Note: developer reported two unrelated `TestRunExecHook_*` failures in `make test-q1` (not touched by this work; check separately).

## 5. Notes for whoever picks this up

- Re-verify against live code and recent commits first; this ticket may sit for a while.
- An agy smoke run on the default card failed once with 772k input tokens and turns=1. Check whether that is normal for agy before trusting agy numbers.

## On hold (2026-09-22)

Parked with the rest of the Dot8 chain. The token benchmark that this ticket's
premise (dense Dot8 for token efficiency) depends on concluded the opposite: braille
characters split under the tokenizer and the penalty outweighs the compression, so
Dot8 must not be the primary LLM input format
(`docs/studies/2026-09-20-dot8-braille-vs-markdown-and-multimodal-context-card-token-benchmarks.md`).
codex and agy legibility remain unverified by a clean canary. Resume once 444's
renderer-pitch fix lands and a clean 3x4 readability canary passes 3/3 on at least
two agents.
