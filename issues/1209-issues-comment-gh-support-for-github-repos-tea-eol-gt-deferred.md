# 1209 — issues comment: gh support for GitHub repos (tea EOL, gt deferred)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 1207 (fj-only issues comment), 1194, 1067

---

## 1. Problem & Motivation

Trial 2026-11-10: `harnez issues comment` (1207) uses `fj` only, so GitHub repos cannot use it.
Owner note (2026-11-10): `tea` is end of life and is replaced by `gt`, which is too young to adopt now
and needs Go 1.27, also too young. So the next CLI to add is `gh`; tea and gt stay out.

## 2. Goal

/goal On a GitHub origin, `harnez issues comment` runs `gh issue comment <n> --repo <host>/<owner>/<repo>
--body-file <file>` after the same preflight as 1207 (PR refused, closed needs `--closed`, target printed,
`--dry-run` prints argv, missing login names `gh auth login`); CLI choice comes from `spec/pr.yaml`
`issue_cli`. Tests use a fake API and a fake gh, never the network. Stop and ask the owner before adding
tea or gt. On a loop or no progress, clear the goal and write a failure report here.

## 3. Acceptance

- GitHub repo: dry run prints the gh argv; PR and closed gates hold; fake gh asserts it was run once.
- Codeberg keeps using fj; tea is not selected for comments even when installed.
- `docs/commands/issue.md` names gh as the GitHub path and records why tea/gt are not used.

## 4. Notes / Unverified

- Whether `issues new` should also drop tea from Codeberg's `issue_cli` order is out of scope; record it.
- `gh issue comment` flag names (`--body-file`) are to be checked against the installed gh.
- Revisit gt when it is mature and its Go requirement is available here.
