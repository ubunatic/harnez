# 347 — Research: replace disableBundledSkills with lean, harnez-focused skill set

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Research

---

## Summary

Issue 316 (`harnez apply --debloat`) trims `permissions.deny` entries, not skills —
skills aren't individually denyable the way tools are; the only lever Claude Code
exposes is the `disableBundledSkills` boolean, which is all-or-nothing. Research what
that toggle actually removes, whether harnez-driven projects lose anything they use by
flipping it on, and whether harnez should ship its own lean, harnez-focused skill
replacements for the handful of bundled skills this workflow actually relies on.

## Background

From live `claude -p "/context"` measurements during 316's work (see
`docs/studies/2026-09-15-debloat-context-usage-measurement-and-cli-flag-comparison.md`),
memory files and skills are a non-trivial share of baseline context (skills ~3.1k tokens
in a clean test project), and a follow-up assessment identified several *built-in*
skills that appear unused in typical harnez-driven work: `lmcoder` (local LLM lifecycle
— no local-model work in this project), `keybindings-help`, `claude-api` (harnez builds
tooling *for* agents, not LLM-calling apps), `schedule` (cloud cron routines — pairs with
the `Cron*` tools already deny-listed in issue 316), `design`/`dataviz` (redundant once
`disableArtifact` is set, since harnez never publishes web artifacts), and `run`
(launch-and-screenshot — harnez is a headless CLI, nothing to screenshot).

`disableBundledSkills` is the only mechanism currently in `internal/claude/debloat.go`'s
`DebloatOptions` that touches skills at all, and it's a single blanket toggle — it can't
selectively keep the skills this project's own workflow actually depends on (`sprint`,
`lean-sprint`, `issue`, `review`, `commit`, `tool-feedback-protocol`, etc., all
user-defined/project skills already unaffected by this toggle since they aren't
"bundled") while dropping the built-in ones nobody in this workflow touches.

## Research questions

1. **What exactly does `disableBundledSkills: true` remove?** Confirm against a real
   installed Claude Code version (same rigor issue 316's review demanded for
   `permissions.deny` — don't assume from the flag name). Does it remove only Anthropic's
   built-in skill set (`dataviz`, `code-review`, `run`, `claude-api`, etc. — the ones
   listed as "Built-in" in `/context`'s Skills table), or does it also affect
   user-installed/project skills, plugin skills, or skill *discovery* mechanisms more
   broadly?
2. **Which built-in skills does a harnez-driven session actually use?** Audit real usage
   (harnez's own session history/telemetry if available, or a deliberate multi-session
   sampling period) rather than guessing from names. Cross-check against the candidate
   "unused" list above (`lmcoder`, `keybindings-help`, `claude-api`, `schedule`,
   `design`, `dataviz`, `run`) and the ones likely still used (`code-review`,
   `security-review`, `update-config`, `fewer-permission-prompts`, `init`, `simplify`,
   `loop`).
3. **Is there a middle ground between "keep everything" and `disableBundledSkills:
   true`?** Investigate whether Claude Code exposes any per-skill enable/disable
   mechanism (skill-level settings, an allowlist/denylist analogous to
   `permissions.deny`, or a marketplace/plugin-scoped toggle) before assuming the binary
   toggle is the only lever.
4. **If no selective mechanism exists, is a harnez-authored replacement skill set
   worthwhile?** For any bundled skill found genuinely useful in harnez work (e.g.
   `code-review`, `simplify`), scope a minimal harnez-flavored equivalent skill (shorter
   description/instructions, scoped to this project's actual conventions instead of
   generic ones) that could ship alongside `disableBundledSkills: true`, so the
   all-or-nothing tradeoff doesn't cost real capability.
5. **Token/behavior cost-benefit**: measure actual context savings from
   `disableBundledSkills: true` alone (same `claude -p "/context"` methodology as issue
   316) before proposing it as a `--debloat` toggle default or new preset member —
   `minimal`'s near-zero real-world saving in 316 is a reminder not to assume savings
   without measuring.

## Definition of done (research ticket)

- Documented answer to each research question above (case study in `docs/studies/` or an
  update to this ticket, per this project's evergreen-doc conventions).
- A go/no-go recommendation: keep `disableBundledSkills` as the existing blunt toggle,
  build harnez-authored lean replacements for specific skills, or find/use a selective
  per-skill mechanism if one exists.
- If replacements are recommended, file separate implementation tickets per skill or per
  batch rather than scoping code changes into this research ticket.

## Related

- Issue 316 — `harnez apply --debloat` / `status --debloat` / `revert --debloat` (the
  `permissions.deny`-based mechanism this ticket's skill-level research complements).
- `docs/studies/2026-09-15-debloat-context-usage-measurement-and-cli-flag-comparison.md`
  — measurement methodology and prior findings this ticket should reuse.
