# 857 — Support external issue providers (GitHub, GitLab, Codeberg, Trello, Jira)

**Status**: Backlog
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 853, 854 (config gating and safety notice), 856

---

/goal harnez agents can work with an external issue provider (GitHub, GitLab, Codeberg/Forgejo, Trello,
Jira) through a provider setting in `.harnez/` config, while the file-first `issues/` tracker stays the
recommended default. Start with a design study; stop and report before building any provider integration.

## Notes

- 854 only gates harnez tracker features and tells agents when a repo does not use them; it does not
  integrate other trackers.
- Open questions: one provider abstraction behind `harnez issues`, or provider-specific skills; offline and
  file-first guarantees; auth handling.
