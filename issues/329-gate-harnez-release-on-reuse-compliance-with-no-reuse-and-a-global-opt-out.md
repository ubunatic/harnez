# 329 — Gate `harnez release` on REUSE Compliance, with `--no-reuse` and a Global Opt-Out

**Status:** Open  
**Category:** Release Pipeline / Compliance  
**Related:** [internal/release/runner.go](../internal/release/runner.go), [cmd/harnez/release.go](../cmd/harnez/release.go), [docs/practices/GoRelease.md](../docs/practices/GoRelease.md)

---

## 1. Problem & Context

`termaid` shipped v0.1.5 with zero REUSE (https://reuse.software) compliance —
no `LICENSES/` directory, and only 8/73 files carrying
`SPDX-FileCopyrightText`/`SPDX-License-Identifier` tags — and `harnez release`
happily built, signed, and published it anyway. `reuse lint` caught it only
after the fact, in a separate manual step, on a repo that had already cut a
tagged release.

`harnez release`'s `runPreflight` (`internal/release/runner.go:268-302`)
already gates the release on tool availability (`git`, `minisign`, `fj`,
`goreleaser`/`make`) before any build/tag/publish step runs. REUSE compliance
belongs in that same preflight gate: a project either is or isn't
REUSE-compliant *before* release, and finding out after a tag is pushed and
signed is too late to matter.

## 2. Proposed Solution

1. **Gate**: In `runPreflight`, if `reuse` is on `PATH` and the target repo
   looks REUSE-managed (has a `LICENSES/` directory, or is opted in via
   config — see below), run `reuse lint` and fail the release with its
   output on non-zero exit, before any build/tag/sign/publish step.
2. **Per-invocation opt-out**: `--no-reuse` flag on `harnez release`
   (`cmd/harnez/release.go`, alongside the existing `--skip-*` flags) skips
   the compliance check for that one invocation only — for a release someone
   needs to cut *now* despite a known, accepted compliance gap.
3. **Global opt-out**: not everyone uses REUSE at all, and a tool-availability
   check alone (`reuse` not on `PATH` -> skip, already implicit) isn't
   sufficient — a user may have `reuse` installed for unrelated projects but
   never want it invoked by `harnez release`. Add a durable, user-level
   config switch (harnez has no existing global settings file per
   `internal/resolve/resolve.go:183`'s `~/.harnez/sessions` /
   `internal/feedback/feedback.go:60`'s `~/.harnez/feedback` convention —
   this ticket would be the first consumer of a general `~/.harnez/config.*`)
   e.g. `release.require_reuse: false`, that disables the gate for every
   project, every invocation, without needing `--no-reuse` each time.
4. Decide and document precedence: CLI flag > global config > repo
   auto-detection (has `LICENSES/`) > default (gate on when `reuse` is
   installed and the repo looks REUSE-managed; otherwise skip silently, no
   new hard requirement on `reuse` for every project).

## 3. Open Questions

- Should the gate also apply when `reuse` is not installed at all — i.e.
  should `harnez release` ever *require* installing `reuse`, or is "skip
  silently if the binary is missing" always correct regardless of the new
  global config? (Leaning: never require installing it; the global config
  only ever narrows an already-opt-in check, never widens it to force an
  install.)
- Where does the new global config file live, and does it need a schema
  shared with other harnez settings, or is a single-purpose
  `~/.harnez/release.yaml` acceptable for now? First consumer of general
  global config — worth checking whether another in-flight ticket already
  wants one before inventing a second mechanism.
- `--continue` (resume a previous release without re-tagging): should a
  resumed release re-run the REUSE gate, or only the initial invocation?

## 4. Verification Plan

- Unit test: `runPreflight` fails with a non-zero-exit `reuse lint` output
  when a fixture repo has a `LICENSES/` dir and missing headers, and
  `--no-reuse` suppresses that failure.
- Unit test: global config `release.require_reuse: false` suppresses the
  gate without needing `--no-reuse` on the command line.
- Manual: reproduce the termaid v0.1.5 scenario (before the follow-up REUSE
  compliance commit) against a `harnez release` build with this gate enabled,
  and confirm it now fails preflight instead of publishing.
