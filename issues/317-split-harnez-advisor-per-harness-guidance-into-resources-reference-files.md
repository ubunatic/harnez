# 317 — split harnez-advisor per-harness guidance into resources/ reference files

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: issue 297 (same on-demand-splitting idea, generalized to
`docs/lang/` docs instead of skills — solve once, generically, if picked up)

---

## Summary

`docs/commands/HarnezAdvisor.md` (the `harnez-advisor` skill, issue 303) inlines all
four harness bullets — Claude, Codex, AGY/Gemini, Prime — directly in the one
`SKILL.md` body that gets loaded into context whenever the skill is relevant. As
harness-specific detail grows (e.g. the Codex CLI-invocation/reasoning-effort
finding added while working issue 316's advisor call), that body keeps growing
even though a given session only ever needs the guidance for the harness it's
actually running.

The `docup` skill already solves exactly this with `config.yaml`'s `resources:`
mechanism:

```yaml
skills:
  - name: docup
    file: docs/commands/Docup.md
    resources:
      - source: docs/commands/DocupTesting.md
        target: references/DocupTesting.md
      - source: docs/commands/DocupArch.md
        target: references/DocupArch.md
```

Companion files are copied to `references/<name>.md` under the installed skill
directory (in every harness target) and read on demand; only the pointer lives
in the always-loaded `SKILL.md` body.

## Proposed change

- Keep `docs/commands/HarnezAdvisor.md` as the short orchestrator-level contract:
  compatibility rules, evidence labeling (`measured`/`reported`/`estimated`/`unknown`),
  lifecycle policy, and one-line pointers per harness.
- Add per-harness reference docs, e.g.:
  - `docs/commands/AdvisorClaude.md`
  - `docs/commands/AdvisorCodex.md`
  - `docs/commands/AdvisorGemini.md`
  - `docs/commands/AdvisorPrime.md`
- Move the current inline bullets (and the Codex CLI-invocation detail from issue
  316's session) into their respective files.
- Add a `resources:` list to the `harnez-advisor` entry in `config.yaml` mapping
  each to `references/Advisor<Harness>.md`.

## Caveat

This does not reduce files on disk — all four reference docs still get copied to
every harness's skill directory regardless of which harness is actually running
there (same as `docup`'s two references are copied everywhere). It only removes
the inlined boilerplate from the always-loaded `SKILL.md` body, which is the
actual token-bloat concern this ticket is about.

## Verification

- [ ] `docs/commands/HarnezAdvisor.md` no longer inlines per-harness CLI/session
      detail — only short pointers to `references/Advisor<Harness>.md`.
- [ ] `harnez apply` copies all four reference files under
      `references/` in `~/.claude/skills/harnez-advisor/`,
      `~/.codex/skills/harnez-advisor/`, `~/.gemini/skills/harnez-advisor/`,
      and `~/.prime/agent/skills/harnez-advisor/`.
- [ ] `go test ./...` passes; `make status`/`make apply` idempotency holds.
