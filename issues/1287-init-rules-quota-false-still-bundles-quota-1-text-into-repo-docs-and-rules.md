# 1287 — init: rules.quota false still bundles Quota-1 text into repo docs and rules

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: issue 944 (Quota-1 counting), issue 381 (init discards local edits in bundled docs), taia commits 9562c56 and 725ce93

---

## 1. Goal

/goal A repo that sets `rules.quota: false` in `.harnez/repo.yaml` gets no Quota-1 text from `harnez init`: no `Quota.md`, no `test-q1` target or `--quota-1` advice in the bundled rules and docs, and no Quota-1 enforcement by hooks or `harnez exec`. Verify it with an init test on a scratch repo. Stop and report when an owner decision is needed, for example whether the opt-out also turns off the hooks.

## 2. Problem (taia, 2026-11-25)

The owner removed Quota-1 from taia. Setting `rules.quota: false` and running `harnez init -d .` dropped `Quota.md` from `.harnez/rules/Index.md`, but a lot was left behind:

- **`.harnez/rules/Quota.md` stays on disk.** init stops listing the file but never deletes it, so the repo had to remove it by hand.
- **Bundled files still name Quota-1 or `test-q1`, and init rewrites them on every run, so hand edits don't last:**
  - `.harnez/rules/Tools.md:48`: "`HTO=0 make test-q1`".
  - `docs/AgenticLoop.md:15`: "Hung quota-1 tests: `harnez clean procs q1 --kill`".
  - `docs/Bash.md:86`: "`HTO=0 make test-q1`".
  - `docs/GoRelease.md:137`: "passing `make lint` plus `make test-q1`".
  - `docs/Spec.md:67`: init validates a `test-q1` target. Check whether this is still true when the opt-out is on.
- **Skills and agent prompts still assume Quota-1.** The installed lean-sprint, factory-sprint and `/disable` skills, plus the prompts `harnez agent` gives developers, still say to use `make test-q1` and count runs. Agents in an opted-out repo keep following a rule that repo no longer has.
- **Unclear: is the opt-out enforced at runtime?** Check whether `harnez exec --quota-1` and the session hooks read `rules.quota`, or whether they enforce Quota-1 anyway.

## 3. Ask

- When `rules.quota` is false:
  - init deletes a `Quota.md` it generated, using the bundled-file marker, so local files are never deleted.
  - Bundled rules and docs use the repo's normal test target, for example `make test-short` or `make test`, in place of `test-q1` and Quota-1 wording. Either template these lines or drop them.
- Skills and agent prompts take the test command from the repo's config instead of hard-coding `make test-q1`.
- State in one place what `rules.quota: false` turns off: init text, hooks, `harnez exec --quota-1` and the developer prompts.

## 4. Verification

- **Init test:** a scratch repo with `rules.quota: false`, then `harnez init`. Afterwards `rg -i 'quota.?1|test-q1'` finds nothing in `.harnez/rules/` or the bundled docs, a stale generated `Quota.md` has been removed, and a second init is idempotent.
- **Default repo:** a repo without the opt-out still gets the full Quota-1 text.
