---
kind: sprint
date: 2026-11-24
tickets: [1275, 1266, 1270, 1279]
branch: sprint/1275-1266-1270
---

# Two-Agent Lean Sprint: Three Harnez Headaches (#1275, #1266, #1270/#1279)

## Scope & `/decide` Choices

Three active P2 harnez headaches selected for a 3-milestone lean sprint + release:

1. **M1 (#1275 — Tier-aware Quota-1)**: Allow upgrading from a narrower/shorter test run (`go test -short`, `make test-short`, targeted packages) to a broader/full run (`make test-q1`) without intervening source edits, while keeping repeat same/lower-tier runs blocked; update `Quota.md` guidance.
2. **M2 (#1266 — `agent resume` unset `context_tokens == 0`)**: Treat never-recorded `context_tokens == 0` like unknown `-1` (skip compaction, warn, dispatch resume) instead of refusing and destroying session continuity.
3. **M3 (#1270 + #1279 — Runner-mode `--dry-run`, read-only `OpenSessionStore`, actionable error route)**: Skip session store open in `--dry-run` under runner mode, allow read-only `OpenSessionStore` (`agent list`/`show`/`status`) while keeping mutating `NewSessionStore` guarded, and name `mcp__taia__background_task` in runner-mode mutation errors.

## Architect Refinement (`argon:med`)

- Dispatched `argon-headaches` (`agy:argon:med`, `--role advisor`, read-only): finished in 8m55s (5.33M tokens incl. cached), rated 5/5 and cleaned up.
- Verified live-code `file:line` citations, caller audits, doc sources/copies, and existing locked tests for all 3 milestones; Searcher skipped per lean-sprint §1.
- Key architect catches:
  - M1 (#1275): `TestRunExecWrapper_PlainGoTestRecordsQuota1State` currently asserts `make test-q1` is blocked after `go test -run Foo .`; `config.yaml` + `.harnez/rules/Quota.md` + `docs/commands/lean-sprint*.md` must stay in sync with `internal/claude/quota1_init_test.go` anchors.
  - M2 (#1266): `VerifyCompaction` (`after <= 0`) stays strictly enforced; only `EnsureContextUnderThreshold` (`<= 0`) + `agent_run.go` warning change.
  - M3 (#1270/#1279): `listCmd`/`statusCmd`/`waitCmd` in `cmd/harnez` currently call `ac.store()` (`NewSessionStore`) instead of `OpenSessionStore`, and two runner-registry tests assert `OpenSessionStore`/`list` fail.

## Milestones

- **M1 (#1275 — Tier-aware Quota-1)**: delivered by `dev-headaches` (`agy:hydrogen:med`, 6m09s, 988k tokens, commit `01fee008`). Host + `rev-m1-1275` (`codex:sol:med`, 1m58s, 47k new tokens, rated 5/5) caught 3 `[MED]` items (`-buildvcs` boolean flag, `-test.short`/`-test.run` aliases, duplicate bullets in `.harnez/rules/Quota.md`) and 2 `[LOW]` items (`make -l 1.5`, extra tests) — folded into M2 pre-work.
- **M2 (#1266 — `agent resume` zero `context_tokens` + M1 pre-work)**: delivered by `dev-headaches` (`agy:hydrogen:med`, 4m28s, 1.75M tokens, commit `80d0efda`). Reviewed by `rev-m2-1266` (`codex:sol:med`, 1m21s, 50.2k new tokens, rated 5/5): **Ready to accept** (`0 [HIGH]`, `0 [MED]`).
- **M3 (#1270/#1279 — Runner-mode `--dry-run`, read-only `OpenSessionStore`, actionable errors)**: delivered by `dev-headaches` (`agy:hydrogen:med`, 11m49s, 5.20M tokens, commit `d09a7d6a`). Reviewed by `rev-m3-1270` (`codex:sol:med`, 2m26s, 54.1k new tokens, rated 5/5): caught `[HIGH]` `OpenSessionStore` writable store + `waitForAgent` `store.Save` stale repair in runner mode, plus `[MED]`/`[LOW]` test gaps.
- **M3 fixup (`b9758e8f`)**: delivered by `dev-headaches` (`agy:hydrogen:med`, 7m50s, 8.27M tokens): enforced `readOnly: true` + `requireWritable()` across all `FileSessionStore` write/lock paths, skipped stale-session `store.Save` in `waitForAgent`, fixed `agent_send.go` `QueueMessage` to use `NewSessionStore`, and added immutability + store-spy tests. Reviewed by `rev-m3-fixup` (`codex:sol:med`, 1m12s, 62.2k new tokens, rated 5/5): **Ready to accept** (`0` blocking findings).

## Final Verification & Teardown

- `make install`: updated `~/.local/bin/harnez`.
- Live smoke: `TAIA_SESSION_DIR=... ~/.local/bin/harnez agent start --dry-run --role developer --model claude:haiku:high --name dry -p "x"` printed preview + uniqueness note without opening the host registry.
- `HTO=0 make test`: exited 0 (`test-pi`: 115 pass / 0 fail / 17 skipped; `gofmt`, `go vet`, and all Go packages green).
- All 5 sprint agents (`argon-headaches`, `dev-headaches`, `rev-m1-1275`, `rev-m2-1266`, `rev-m3-1270`, `rev-m3-fixup` — 6 sessions total) rated 5/5 and deleted.
- Closed tickets `#1275`, `#1266`, `#1270`, `#1279`.
