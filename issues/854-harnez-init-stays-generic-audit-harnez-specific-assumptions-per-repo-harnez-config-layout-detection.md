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
| `internal/claude/init.go:1115` | `.harnez/rules` + managed AGENTS.md sections | default | remaining rule categories are shared; quota is gated by `rules.quota` (M2) |
| `internal/claude/init_phases.go:147` | adds container-test target to an existing Makefile | `make.container_test` true | gated by repo config; detection requires an existing target (M2) |
| `internal/claude/init_phases.go:80` | copies selected language docs and injects their Makefile targets | `--docs`, existing AGENTS refs, or auto-detection | `docs.copies` records first detection; it does not gate copy behavior; scratch Makefile still receives Make.md and target changes |
| `internal/claude/init_phases.go` Makefile reconciliation | scaffolds the help target in a selected Make doc's Makefile block | copied Make doc + existing Makefile | still adds `.PHONY`/help scaffold to foreign Makefiles; no repo config gate |
| `internal/claude/init.go:1186`, `init_quota.go:16` | quota rules + `test-q1` target | `--quota-1`; existing setup detected in repo config | quota rule inclusion gated by `rules.quota` (M2); explicit flag creates the section/target |
| `internal/claude/init.go:1142` | go.work | `--gowork` | opt-in flag; not represented in repo config |
| `internal/claude/init.go:798,865` | `--all` eligibility and docs detection | detected project markers / docs config | first layout is recorded in repo config; eligibility and doc selection still use detection |
| `cmd/harnez/issues.go:351` | issue verbs | invocation | misread a foreign `issues/` format; detect format |

Related backlog: 494 (component selection in init, overlaps; reconcile here), 568 (split copyable
IssueTracking/AgenticLoop docs into generic practice + harnez rules; the prose side of this ticket).
Skills: 856.

## Per-repo config design (M1, before any gating)

No per-repo config exists yet (2026-10-22). Today there is only the embedded `config.yaml` with
`spec/schemas/config.schema.json` and the user's global `~/.harnez/config.yaml`; both are flat, mixed-concern
and global. M1 delivers the per-repo file `.harnez/repo.yaml` (name decided by the user, 2026-10-22; avoids confusion
with the global `config.yaml`) with its own JSON schema in `spec/schemas/`, a `version` key,
one top-level key per feature area so each area can grow without reshaping the file, and only the keys the
gating needs. Draft:

```yaml
version: 1
issues:            # file-first tracker (recommended default)
  tracker: harnez  # harnez | none
  merge_driver: true
docs:
  studies: true    # docs/studies/ + generated docs/README.md index
  copies: [Go, Bash]
rules:
  quota: false
make:
  container_test: false
```

Absent file = today's detection, written once on first init. Unknown keys fail schema validation.

## Milestones (sprint 2026-10-22)

- **M1 (repo.yaml)**: schema in `spec/schemas/`, loader, first-init detection writes the file once, later runs
  read it; flags override and are recorded. This repo gets a committed `repo.yaml` matching today's behavior.
- **M2 (gating + notice)**: `make.container_test` gates `init_phases.go:147`; `rules.quota` gates the quota
  rule; when `issues.tracker: none` or `docs.studies: false`, init writes the safety notice into
  `.harnez/rules/Index.md`. Issue-git gating itself stays in 853 but must read `issues.*` from this file.
- **M3 (docs)**: `docs/CLIDesign.md` init flow and `docs/Spec.md`/schema docs describe repo.yaml; audit table
  above updated with what is now gated.

**M1 delivered (`85f6732a`)**: repo.yaml schema, loader with validation, detect-once, flag recording; this
repo's repo.yaml committed; tests green, self-init clean.

**M2 Pre-Work / Required Refinements** (from M1 review):
- `make.container_test` detection uses "Makefile exists"; detect "Makefile already has the container-test
  target" instead, so new repos default to false.
- `rules.quota` detection is true when `.harnez/rules/Quota.md` exists, which init writes for every repo;
  detect the quota section / `test-q1` target only.
- Detection tests for each key: foreign `issues/` (no marker) → `none`; harnez-marked → `harnez`; Makefile
  without target → `container_test: false`; existing repo.yaml is not rewritten without a flag.

**M3 Pre-Work / Required Refinements** (from M2 review):
- `writeRules` takes a plain `*RepoConfig` argument (nil allowed), not a variadic; its caller and nil case
  test are updated.
- The audit explicitly records Makefile help scaffolding and doc copies into foreign repos, and marks
  the container-test and quota gates added in M2.

**M3 delivered**: init flow, spec/config distinction, and audit table documentation updated. Ticket remains
open for follow-up divergences, including 853 and the ungated Makefile/doc-copy behavior above.

**M2 delivered (`b2e5e5ad`)**: detection fixed, container-test and quota gated by repo.yaml, notices written to
Index.md. Host scratch check (Makefile + foreign `issues/README.md`): repo.yaml `tracker: none`, no
container-test target, both notices present. Still seen there: the merge driver and `.gitattributes` are
installed although repo.yaml says `merge_driver: false` (853, next), and init adds the `.PHONY`/`help`
scaffold to the foreign Makefile and copies `docs/Make.md` (detected via `copies: [make]`).

**M3 Pre-Work / Required Refinements**:
- `writeRules(..., repoConfigs ...*RepoConfig)`: replace the variadic with a plain `*RepoConfig` parameter
  (nil allowed) and update callers/tests.
- Audit table: add rows for the Makefile help scaffold and doc copies into a foreign repo, mark what M1/M2
  now gate.

Checks: scratch repo with a Makefile and no `issues/`/`docs/`: init adds no container-test target, writes
repo.yaml with `tracker: none`, writes the notice; this repo: `make install && harnez init -d .` leaves a
clean `git status`.

## Open questions

- Config format and name (`.harnez/config.yaml` vs extending an existing file); schema in `spec/`.
- Migration for repos already initialized: write the config from what is installed today.
