---
kind: sprint
date: 2026-10-30
issue: 1037
---

# Lean sprint 1036/1037: `/pm` skill, auto-trigger for focus and the sprint skills, searcher compaction

## Goal
Make `focus`, `lean-sprint` and `factory-sprint` auto-triggerable. Add the searcher-compaction rule (1036). Add a
`/pm` skill that keeps a host session in product-manager mode (1037, first proposed as `/master`).

## Agents
- Searcher `cheap` → `codex:luna:low` (76 s, 125k new, 1.4M cached). It found the Claude-only debloat override
  path, the `disable-model-invocation` frontmatter in the factory-sprint source, the canonical skill
  sources and the registration steps. Two of its line ranges were slightly off.
- Developer `lean` → `claude:haiku-5-5`: plan turn (68 s, 354k incl. cached), write turn (4 min 31 s, 2.56M incl.
  cached). The plan turn corrected the searcher's line ranges and found that config `skills:` entries are copied
  verbatim (`makeExplicit` only applies to registry and bundled installs), so no Go change was needed.

## Milestones
- M1 (auto-trigger) `2ff9264f`: removed the three `user-invocable-only` overrides and the factory-sprint frontmatter;
  rewrote the descriptions with "use when" triggers.
- M2 (1036 compaction) `5026b0ac`: one compaction bullet each in factory-sprint Phase 1 and lean-sprint §1. The
  threshold trigger, the resume hint and the live Pre/Post check were parked in 1038.
- M3 (`/pm`) `53a39c82`: `docs/commands/pm.md` (42 lines, references the invariants instead of copying them);
  config entry plus a `pm: user-invocable-only` override. 145 closed as superseded.

## Verification
- Host diff review: config, the four command docs, the tickets. No Go files changed.
- Installed copies: `~/.claude/skills/factory-sprint/SKILL.md` has no `disable-model-invocation`, and the new
  descriptions appear in the live skill list.
- `make lint`: exit 2, 213 issues, the same before and after (3 glossary errors in 1031, which predates this sprint).
- `make test-q1`: one failure, `TestGoldenFontAssets` (internal/readcard, stale golden fonts). It predates this
  sprint because the branch changes no Go or font files.
- Open: live `/pm` session check (1037 §4). 1037 stays open until the owner runs it.

## Observations
- The skill names alone settled the name choice: `/pm` beat `/master`, `/conductor`, `/helm` and `/lead`.
- On this machine `~/.claude/settings.json` has no `skillOverrides`. The debloat preset is not applied, so every
  `user-invocable-only` skill, `pm` included, still appears in the model's skill list. Auto-trigger
  descriptions cost about 35 tokens each.
- Reused cheap searchers are cheap per turn (76 s) but heavy on cached input; 1036's rule targets that.
