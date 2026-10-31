# 1069 — Add trial skill for concise 10-5 capability audits (10 things we can do, 5 things we cannot)

**Status**: Closed — delivered /trial capability audit skill with anti-overclaim rules, test coverage, and documentation
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [retro](file:///home/uwe/.gemini/skills/retro/SKILL.md), [study](file:///home/uwe/.gemini/skills/study/SKILL.md), [focus](file:///home/uwe/.gemini/skills/focus/SKILL.md)

---

## 1. Problem & Motivation

During long development sessions, sprints, or feature milestones, users frequently request a high-leverage checkpoint summarizing what was actually unlocked versus remaining boundaries:
- **Ten Things We Can Do Now**: Recent capabilities, verified bug fixes, new CLI commands, and proven workflows.
- **Five Things We Cannot Do Yet**: Remaining gaps, unverified canary assumptions, open tickets, or blocking bugs.

Today, this capability check (the "10-5 trial") is conducted ad-hoc in conversation. Packaging this into an invocable `/trial` (or `/audit-10-5`) skill standardizes how agents inspect live git commits, closed tickets, and verified features to produce grounded, evidence-backed capability summaries without fluff or hallucinated claims.

## 2. Technical Specification

1. **Skill Definition (`skills/trial/SKILL.md`)**:
   - Invocable via `/trial` or `/trial <scope>` (e.g. `/trial today`, `/trial sprint`, `/trial <ticket>`).
   - Grounding rules:
     - Must inspect recent git commit history (`git log --since=...` or `git log -n ...`) and recently closed tickets (`issues/CLOSED.md`).
     - Extracts exactly 10 concrete, delivered capabilities with verifiable evidence (linking commit hashes or tickets).
     - Extracts exactly 5 clear boundaries, open blockers, or pending follow-ups (linking open tickets or unmerged PRs).
   - Enforces Concise Mode: verbatim commands, no opening/closing pleasantries, sharp contrast between what works and what does not.

2. **Integration into Harnez Skills**:
   - Register `/trial` in the skills registry (`internal/skillreg` or installed skill catalog).
   - Document the `/trial` skill in `docs/practices/AgenticLoop.md` alongside `/evergreen`, `/retro`, and `/study`.

## 3. Goal & Acceptance Plan

/goal Author and register the /trial skill to produce grounded 10-5 capability audits from live git history and issue tracker state, verified with automated skill validation tests, or stop and report when blocked on user input or denied permission.

1. **Author `SKILL.md`**: Create the skill instruction file with clear frontmatter, workflow guidelines, and the 10-can / 5-cannot output template.
2. **Registration & Docs**: Add `/trial` to skill definitions and relevant documentation.
3. **Verification**: Run a live trial invocation across recent commits to verify that it outputs 10 accurate capabilities and 5 grounded limitations with exact file/commit links.

---

## 4. Implementation Notes

- Skill source file: `docs/commands/trial.md` (registered in `config.yaml` as `trial`, `user-invocable-only` under the skill presets; covered by `internal/claude/claudeskills_test.go`; documented in `docs/practices/AgenticLoop.md` and its lite copy).

## 5. Live Verification: `/trial today` (2026-10-31)

Evidence: `git log --since="midnight" --oneline` (77 commits), `harnez find -d . issues closed` (newest 10), `git log --since="midnight" -- issues/`. Verified at HEAD by running `go test` on the named Go tests, `bash scripts/test-pi.sh` (130/130 pass) and `harnez ... --help`. One item per ticket.

# ⚖️ Capability Trial: harnez, 2026-10-31

### Nine Verified Things We Can Do Now

(Only 9 verified; no padding. The pixel-based golden font comparison, 2414bf73 / 1004, was dropped as an internal test refactor.)

1. **Time-limited model defaults per effort tier** — `harnez models set --until=HH:MM|--hours=<n>` (also `harnez agent models set`); `TestAgentModelsSetFlagValidationAndExpiry`; commit 969632f0, ticket 1056.
2. **Selectable agent context modes** — `harnez agent start --context-mode plain|no-skills|default` (Pi driver only); `TestAgentStartContextModePassThrough`; commit c7830529, ticket 1027.
3. **Detached-HEAD warning at session start and commit guard** — `TestCheckDetachedHead`, `TestRunClaudeInstructionsHookDetachedHeadWarning`; commit bc15f0ae, ticket 1065.
4. **Issue-index lock files ignored and cleaned up** — `harnez init` writes the tracked gitignore entry; `TestRunInit_IgnoresIssuesLockFilesInTrackedGitignore`, `TestReserveFreshCloneLeavesGitStatusClean`; commit 2fbe34aa, ticket 1066.
5. **Private exclusive capture of Pi background task output** — `test/pi/capture-store.test.ts` ("private exclusive captures contain exact bytes..."); commit c04ce40c, ticket 1021 (still Open: further work remains).
6. **Batched background-task completions** — `test/pi/compaction-delivery.test.ts` ("first arrival fixes the window; ..."); commit 8eb43a12, ticket 1044 (still Open).
7. **Manual-compaction completions are held and delivered in order** — `test/pi/compaction-delivery.test.ts` ("manual compaction holds FIFO until the end hook and the guard both clear"); commit 85293bf3, ticket 1047 (still Open: full auto-compaction no-loss has remaining gaps).
8. **Collapsed task result output until expanded** — `test/pi/background-tasks.test.ts` ("default capture is visible in completion and result; ..." asserts collapsed/expanded rendering); commit 4a56d4e0, ticket 1046 (still Open).
9. **Scoped 10-5 capability audits** — `/trial [today|sprint|<ticket>]`, `docs/commands/trial.md`, checked by `internal/claude/claudeskills_test.go`; commits a3028bf0 and dd6e035e, ticket 1069.

---

### Five Things We Cannot Do Yet

1. **Pin docs, skills and tools per role** — plain mode exists but role profiles do not; open [1068](1068-adopt-plain-mode-for-general-tasks-and-create-role-profiles-to-pin-docs-skills-and-tools.md).
2. **Use xhigh/max effort tiers in model picks** — `claude:haiku:xhigh` is unsupported; open [993](993-support-xhigh-and-max-effort-tiers-e-g-claude-haiku-xhigh.md).
3. **Background a running manual Pi command with Ctrl-B** — Pi 1.4.0 has no running shell transfer API; blocked [1048](1048-background-manual-pi-commands-with-ctrl-b.md).
4. **Guard Pi session reload while background tasks run** — Pi 1.4.0 exposes no cancellable pre-reload hook; blocked [1049](1049-guard-pi-session-reload-while-background-tasks-are-running.md).
5. **Refresh the Pi background task extension independently** — global refresh parked from 1051; open [1052](1052-safely-refresh-the-pi-background-task-extension-independently.md).
