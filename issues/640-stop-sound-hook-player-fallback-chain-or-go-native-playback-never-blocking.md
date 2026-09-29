# 640 — Stop sound hook: player fallback chain or Go-native playback, never blocking

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Config & Hooks
**Related**: [[641-research-pure-go-no-cgo-sound-playback-for-notifications]], [[335-support-afplay-audio-notifications-on-macos-in-default-hooks]], config.yaml `hooks`, commit d2467af0

---

/goal Replace the hardcoded `ffplay` Stop hook with a sound command that tries a chain of
installed players and never blocks the agent; verify with a sleeping or missing audio sink; or
stop and report when blocked on a user decision.

## 1. Problem
The Stop hook in `config.yaml` runs `ffplay`. On 2026-09-29 it hung for minutes in two Claude
sessions while the Bluetooth sink was asleep; commit d2467af0 capped it with `timeout 3`. It
still depends on one player being installed, and the file path is Linux-only.

## 2. Direction
- A harnez subcommand (e.g. `harnez notify sound`) that the hook calls, so logic lives in Go.
- It detaches (returns at once) so the Stop hook never waits, with a hard timeout on the player.
- Fallback chain (decided 2026-09-29): first available of `pw-play`, `paplay`,
  `canberra-gtk-play`, `aplay`, `ffplay` (Linux), `afplay` (macOS); the four Linux ones are
  present on this machine. Pure-Go playback is researched separately in issue 641.
- Player list, sound file and timeout belong in `spec/` (AGENTS.md rule), not in Go.
- macOS (merged from issue 335): `ffplay` and the freedesktop sound are absent there; use
  `afplay` with a system sound such as `/System/Library/Sounds/Glass.aiff`. The sound file is
  chosen per OS. Linux configs must keep working unchanged after `apply`.

## 3. Milestones (plan by dev640, agy:flash37:med, reviewed by host 2026-09-29)
Plan: `harnez hook sound` in `cmd/harnez/hook_sound.go` (wired via `hook.go`, no `main.go`
edit); logic in `internal/sound`; `spec/sound.yaml` + `spec/schemas/sound.schema.json` hold
per-OS sound files, player chain (`{file}` placeholder) and timeout. Default mode re-execs
itself detached (`Setsid`, stdio to /dev/null) and exits 0 at once.

### M1 — spec + `internal/sound` with tests
Host refinements (pre-work):
- A player that times out means the sink is stuck: kill it and **stop**, do not try the next
  player (that would stack timeouts and play late). Fall through only when a player is missing
  (`LookPath` fails) or exits non-zero at once.
- Kill the whole process group on timeout (`Setpgid` on the player), not just the pid.
- Tests use fake players (temp scripts on a temp `PATH`), never real audio.

M1 delivered (170018ff, dev640b): spec/sound.yaml, internal/sound (fallback chain, stop on
timeout, process-group kill), 11 unit tests green. Schema file landed in parallel commit 5cbf3efd.
Builds for linux and darwin; not windows (Setpgid), same as the rest of harnez today.

### M2 — `harnez hook sound` command + Stop hook in `config.yaml`
Pre-work:
- Add a test that the timeout kills the whole group: fake player starts a background
  `sleep` child that writes its pid to a file, then hangs; after timeout assert that child pid
  is gone.
- `config.yaml` has uncommitted parallel edits (issue 642). Commit only the Stop-hook line:
  stage it with `git apply --cached <patch>` built from that hunk alone, and check
  `git diff --cached` shows no other hunk before committing. Never `git add config.yaml`.
- Linux behaviour after `harnez apply` must match today (sound plays, hook returns at once).
