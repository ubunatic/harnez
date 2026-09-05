# 016 — `make smoke` convention: live-run targets in Make.md

**Status:** Open

## Context

The bundled `docs/Make.md` standardizes help/build/test targets, but has no
convention for *live verification* — briefly running the real binary against
real inputs. In proctop's initial build session (2026-08-01), `go test`
stayed green while two real bugs existed that only a live run caught:

- name-target resolution picked a childless Firefox `--dbus-service` stub
  over the real browser tree,
- `term.GetSize` returned 0×0 without an error on a fresh pty, silently
  dropping all TUI graph rows.

"Tests green" is weak evidence for tools that read live system state
(/proc, terminals, sockets). proctop documented its pty harness in its own
`docs/Testing.md` + `issues/007`, but the convention belongs in the managed
Make doc so every project gets it.

## Proposal

- Add a `smoke` section to `docs/Make.md`: a `smoke: ⚙️` target that runs
  the built binary for a few seconds against a real target and asserts
  basic output (grep for expected lines / exit code).
- Recommended shape: `smoke` depends on `build`, uses `timeout`, redirects
  to a file before asserting (pipes + timeout produce misleading empty
  output), and for TUIs drives a pty (Python `pty` harness — `script(1)`
  is not installed on this host).
- Convention note for agents: run `make smoke` (when present) before
  declaring a feature working; it complements `make check`, not replaces it.
- Optionally cross-ref from `docs/Canary.md` (probe-before-build) since the
  spirit is the same: verify against reality, not assumptions.

## Related

- proctop `docs/Testing.md` (pty harness, pitfalls) and proctop issue 007
  (make smoke target there)
- [010-smoke-test-agent-visibility.md](010-smoke-test-agent-visibility.md) —
  different "smoke" (claudeconfig's own install check), naming overlap only

---

## Implementation Plan

Docs-and-template only; no Go changes. Current state confirmed while planning:
`docs/lang/Make.md` has sections for help/build/install/check/deployment parity
but **no** `smoke` section; `docs/templates/Makefile` has no `smoke` target;
this repo's own `Makefile:81` already has the canonical example
(`smoke: ⚙️ build` → `bash scripts/smoke-test.sh`); `docs/practices/AgenticLoop.md:28`
already *mentions* `make smoke` as a verification vehicle but nothing defines it.

### Steps

1. **`docs/lang/Make.md` — new `## Smoke target` section**, inserted between
   `## Check target` (line ~99) and `## Deployment target parity` (line ~119),
   since the parity section at line ~146 already forward-references `make smoke`
   and currently dangles.
   Content: the target shape, plus the three hard-won rules from the ticket —
   - `smoke: ⚙️ build` (always depends on `build`, runs the *real* binary);
   - wrap in `timeout`, redirect to a file, then assert on the file
     (`timeout 5s ./bin/app > /tmp/smoke.out; grep -q 'expected' /tmp/smoke.out`)
     — piping directly into `grep` under `timeout` yields misleading empty output;
   - for TUIs, drive a pty (Python `pty` module harness; `script(1)` is not
     installed on this host) — cross-ref `canary-watch-pty/` in this repo as a
     working example.
   Close with the boundary rule: `smoke` complements `check`, never replaces it;
   `check` must stay hermetic, `smoke` is allowed to touch live system state.
2. **`docs/templates/Makefile` — add a `smoke` stanza** after `check-fast`, so
   scaffolded projects get the slot:
   ```makefile
   smoke: ⚙️ build  # live smoke: run the real binary against a real target
   	@timeout 5s ./$(BINARY) --version > /tmp/$(BINARY)-smoke.out || true
   	@grep -q . /tmp/$(BINARY)-smoke.out || (echo "❌ no smoke output" && exit 1)
   ```
   Keep it deliberately trivial and obviously-a-placeholder — a scaffolded smoke
   target that pretends to verify something real is worse than none.
3. **`config.yaml:385` — extend the `make` hint** so the convention reaches
   `AGENTS.md` without a doc read:
   `hint: "⚙️ phony sentinel, self-doc help, build dependency pattern; make smoke = live-run verification"`.
4. **`docs/other/Canary.md` — one cross-ref line** ("probe before build" ↔
   "verify against reality after build"). One sentence, no new section.
5. **Agent-facing rule**: add a bullet to `docs/practices/AgenticLoop.md` Phase 2
   next to the existing line 28 mention — "run `make smoke` when the target
   exists before declaring a feature working". This is the only place the
   *obligation* (vs. the definition) belongs.
6. Verify with `harnez diff` / `make init` that `docs/Make.md` (the generated
   local copy) and `~/.claude/docs/Make.md` re-sync cleanly. No test changes.

### Design decisions

- **Definition lives in `Make.md`, obligation in `AgenticLoop.md`.** Splitting
  them avoids the duplicate-drifting-copy problem; `AgenticLoop.md` already
  references `make smoke` without defining it, so this closes an existing dangle
  rather than inventing a second home.
- **No `harnez` code, no lint check** that a project *has* a `smoke` target.
  Many projects (pure libraries) legitimately have nothing live to smoke; a
  linter would generate false pressure to fake one.
- **Placeholder in the template rather than omission**: the slot's existence is
  the convention; its content is necessarily project-specific.

### Risks / open questions

- The template's placeholder could be left un-customized and give false
  confidence. Mitigate with an inline comment marking it as a stub to replace.
- Naming overlap with issue 010's "smoke" (this repo's own install check) is
  already noted in Related; the `Make.md` section should not reference 010.
- `docs/Make.md` at repo root is a *generated* copy — edit `docs/lang/Make.md`
  only, then re-run `init`. Editing the root copy is the likely mistake here.

### Scope

**Small** — 4 file edits (one doc section, one template stanza, one config hint,
two cross-ref lines), no code, no tests.
