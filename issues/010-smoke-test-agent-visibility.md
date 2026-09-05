# Smoke-test that agents can see installed skills and commands

**Status:** Open — blocked on wayreel#11 (V step)

**Severity:** Low — correctness gap, not a data-loss bug

## Problem

The integration test and smoke script verify that claudeconfig *writes* the
right files to the right paths. They do not verify that the target agents
(agy, codex, claude) actually *see* those files — i.e. that the install
paths match what each agent reads at startup and surfaces in its TUI.

A path regression (wrong target dir, wrong filename convention, wrong
directory structure) would pass all current tests but silently break agent
discovery.

## Plan

Use wayreel reels with `V contains=` steps to drive each agent's TUI,
trigger its skill/command listing, and assert expected names appear.

Draft reels are already in `reels/`:
- `reels/smoke-agy.reel` — agy `/skills` → verifies `evergreen`, `domain-modeling`
- `reels/smoke-codex.reel` — codex `/skills` → same checks
- `reels/smoke-claude.reel` — claude `/usage` → verifies installed commands

These reels are spec-complete but cannot run yet: the `V` step is not
implemented in wayreel (tracked in wayreel issue #11).

## What needs to happen

1. wayreel#11 lands (`V contains=` step, shell tee-log capture).
2. Confirm exact slash command for each agent that lists skills/commands
   and produces stdout (needed for the tee approach to work); update reels.
3. Add a `make smoke-agents` target (or extend `scripts/smoke-test.sh`) that
   runs the three reels via `wayreel play` and fails if any `V` check fails.
4. Confirm `smoke-claude.reel` — `/usage` is a UI command that may not print
   to stdout; may need `claudeconfig status` piped inside the TUI instead.

## Related

- wayreel#11 — V (verify) step implementation
- issue#007 — general test coverage gaps

---

## Implementation Plan

### Blocker is cleared

wayreel#11 has landed. `../wayreel/script.go` implements `verifyContains` (:1119),
`verifyOcrContains` (:1140) and the `V` dispatch (:1183); `../wayreel/Reel.md:133-140`
documents three attributes: `contains=`, `ocr=`/`ocr_contains=`, and `file_exists=`.
This ticket is **unblocked** — change Status to `Open` and drop the wayreel#11 note.

### The one real design problem the reels currently get wrong

`contains=` asserts against the **shell stdout tee-log only** — Reel.md states plainly it
"sees only what the shell writes to stdout (not TUI app screen renders)". All three draft
reels use `V contains=` immediately after a TUI slash command (`/skills`, `/usage`), whose
output is a screen render, never stdout. As written they would fail regardless of whether
the skills are actually installed — a false negative that is worse than no test.

Two viable strategies; use **both**, at different layers:

- **Layer A — non-TUI stdout probe (primary, cheap, deterministic).** For each agent, find a
  headless/list invocation that prints to stdout, run it via `$`, and assert with
  `contains=`. This is the assertion that gates CI.
- **Layer B — OCR screen probe (secondary, proves the agent's own TUI surfaces it).** Keep
  the `/skills` interaction and switch the assertions to `ocr="evergreen"`. This is the
  check that actually answers the ticket's question ("does the agent *see* it"), but OCR is
  fuzzy and needs grim + Tesseract, so it must not be the only gate.

### Steps

1. **Determine the stdout listing command per agent** (research, do first — everything else
   depends on it):
   - `agy` — check for a non-interactive skills/commands listing subcommand or flag.
   - `codex` — same.
   - `claude` — `claude -p` with a prompt is available but costs an API call; prefer a local
     listing flag if one exists, else fall back to Layer B only for claude.
   If an agent has no stdout listing at all, that agent's reel is OCR-only; note it in the
   reel header comment so the next reader does not "fix" it back to `contains=`.

2. **Rewrite the three reels** (`reels/smoke-agy.reel`, `reels/smoke-codex.reel`,
   `reels/smoke-claude.reel`):
   - Layer A block first: `$ <listing cmd>` → `P` → `V contains="evergreen"` /
     `V contains="domain-modeling"`.
   - Layer B block: launch the TUI, `$ /skills`, `P 3s`, `V ocr="evergreen"`, then exit.
   - `smoke-claude.reel` additionally asserts an installed *command* name, not a skill, since
     that is the distinct discovery path (`~/.claude/commands/*.md`).
   - Add `V file_exists=` preconditions at the top of each reel (e.g.
     `~/.claude/skills/evergreen/SKILL.md`, `~/.gemini/skills/...`, `~/.codex/skills/...`)
     so a failure distinguishes "harnez never installed it" from "the agent can't see it".

3. **`make smoke-agents` target** in `Makefile`, next to the existing `smoke` target (:81):
   ```
   smoke-agents: ⚙️ build  # drive each agent's TUI and verify installed skills/commands are visible
   	bash scripts/smoke-agents.sh
   ```
   New `scripts/smoke-agents.sh`: for each reel, skip with a clear message if the agent
   binary is absent (`command -v agy` etc.) or if `wayreel` is not installed or `WAYLAND_DISPLAY`
   is unset; otherwise `wayreel play reels/<reel>`. `wayreel play` already exits non-zero on
   any failed `V`, so the script just needs to propagate the worst exit code and print a
   summary.

4. **Keep it out of `make smoke`.** `scripts/smoke-test.sh` is the headless, always-runnable
   gate; agent visibility needs a Wayland session, three third-party binaries, and OCR. Wire
   `smoke-agents` as a separate, opt-in target and mention it in `docs/Canary.md`.

5. **Run it once by hand and confirm with the user.** Per this repo's media/recording rule,
   show the user the actual play output (and any screenshot) before declaring the reels
   correct — OCR assertions in particular need eyes on the first pass.

### Design decisions / tradeoffs

- **`ocr=` is the only mechanism that answers the ticket's actual question**, but it is the
  least deterministic; pairing it with a `contains=` stdout probe and `file_exists=`
  preconditions keeps failures diagnosable.
- **Skip-if-absent rather than fail-if-absent** — this is a hobby workstation; a machine
  without `codex` installed should not fail the target.
- **Do not extend `scripts/smoke-test.sh`** — mixing a headless test with a Wayland/OCR test
  in one script makes the fast gate unrunnable in a container.

### Risks / open questions

- Step 1 may find that none of the three agents has a stdout skills listing, collapsing
  Layer A entirely. Acceptable — the reels then rest on `ocr=` + `file_exists=`, which is
  still strictly better than today's zero coverage.
- OCR misreads on hyphenated names (`domain-modeling`) are plausible; if flaky, assert on the
  simpler `evergreen` only.
- Launching `claude` inside a reel starts a real session and may consume quota; consider
  restricting `smoke-claude.reel` to the `file_exists` + OCR-of-slash-completion path.

### Scope

**Medium** — mostly reel authoring and one shell script; no Go changes. Step 1's research and
the manual verification pass are the real cost.
