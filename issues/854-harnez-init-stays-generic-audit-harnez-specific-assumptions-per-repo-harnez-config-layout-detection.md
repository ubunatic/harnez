# 854 — harnez init stays generic: audit harnez-specific assumptions, per-repo .harnez config, layout detection

**Status**: Open
**Priority**: P2 (Medium) — init must keep working for repos that are not built like harnez
**Severity**: Major
**Category**: Architecture
**Related**: 853 (index files and merge driver), 855 (init wizard), docs/CLIDesign.md (init flow)

---

/goal `harnez init` works for repos of any type: every harnez-specific behavior (issue tracker layout, studies
index, rules dir, doc copies, Makefile, quota rules) is either detected from what the repo already has or
enabled through per-repo config in `.harnez/`, and nothing is imposed by default. Deliver the audit as a table
in this ticket first, then one follow-up ticket per divergence; stop and report when a choice needs the user.

## Background

init has grown alongside this repo's own workflow (issues/, docs/studies/, generated indexes, merge driver,
quota rules). Some of these may now be applied to any repo. 853 is one confirmed case.

## Scope

1. **Audit**: list each thing init writes or installs, its trigger (default, detected, flag), and whether it
   assumes the harnez layout. Mark divergences.
2. **Per-repo config**: `.harnez/` holds the repo's choices (a single config file next to `rules/`), read by
   init, apply-time hooks and `harnez index`; flags override and are recorded.
3. **Detection on first init**: infer the setup from what exists (issues/ format, docs/ layout, Makefile,
   language files) and record it in the config, so later runs don't guess again.
4. Follow-up tickets for each divergence; 853 already covers index files and the merge driver.
5. **Safety notice**: when the config says the repo has no harnez tracker or no indexed studies, init writes a
   short notice into a core file that AGENTS.md always loads (e.g. `.harnez/rules/Index.md` or the managed
   AGENTS.md section): "this repo does not use file-based issues / indexed studies; skip those steps and use
   the repo's own tracker". Skills then need no per-skill detection prose (856).

## Scope decision (user, 2026-10-22)

The file-first tracker stays the recommended default and a core concept; this work does not de-emphasize
it. Scope is: gate what can be gated now behind `.harnez/` config, never fail in a repo that doesn't use
it, and tell agents so via the notice above. Other issue providers are out of scope (857).

## Audit seed (research pass 2026-10-22, re-verify before use)

| Location | What init does | Trigger | Divergence |
|---|---|---|---|
| `internal/claude/init_agent_file.go:62` | AGENTS.md + CLAUDE.md symlink | default | ok (core), list in config |
| `internal/claude/init_agent_file.go:96` | merge driver, hook, excludes | any `issues/` dir | yes → 853 |
| `internal/claude/init.go:1115` | `.harnez/rules` + managed AGENTS.md sections | default | yes: rule categories ungated |
| `internal/claude/init_phases.go:147` | adds container-test target to any existing Makefile | default | yes: should be opt-in |
| `internal/claude/init_phases.go:80` | language docs, templates, Makefile targets | `--docs` / detection | check detection defaults |
| `internal/claude/init.go:1186`, `init_quota.go:16` | quota rules + `test-q1` target | `--quota-1` | ok (flag), record in config |
| `internal/claude/init.go:1142` | go.work | `--gowork` | ok (flag) |
| `internal/claude/init.go:798,865` | `--all` eligibility, doc detection by markers (Makefile, issues/, spec/) | detection | feed into config detection |
| `cmd/harnez/issues.go:351` | issue verbs | invocation | misread a foreign `issues/` format; detect format |

Related backlog: 494 (component selection in init, overlaps; reconcile here), 568 (split copyable
IssueTracking/AgenticLoop docs into generic practice + harnez rules; the prose side of this ticket).
Skills: 856.

## Open questions

- Config format and name (`.harnez/config.yaml` vs extending an existing file); schema in `spec/`.
- Migration for repos already initialized: write the config from what is installed today.
