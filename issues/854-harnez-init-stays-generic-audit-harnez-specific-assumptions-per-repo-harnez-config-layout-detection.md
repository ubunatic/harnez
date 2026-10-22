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

## Open questions

- Config format and name (`.harnez/config.yaml` vs extending an existing file); schema in `spec/`.
- Migration for repos already initialized: write the config from what is installed today.
