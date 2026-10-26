# 966 — Allow short alias for external skill sources, e.g. egm for harnez skill install

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 926 (install repository skills), 631 and 632 (external skill registry, closed), 636 (onboard skills from a URL, backlog)

---

/goal Users can install an external skill repository by a short alias: `harnez skill install egm` installs the
skill source `https://github.com/google/elephant-goldfish-model` the same way as `harnez skill install <url>`.
Done when the alias is configured, resolves to the same pinned install, is covered by tests, and `harnez skill
list` shows it. Or stop and report when the alias needs an owner decision (naming, where aliases live).
Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation
Installing external skills needs the full repository URL every time. A short, user-defined alias (here `egm`
for `https://github.com/google/elephant-goldfish-model`) makes the install command short and repeatable.

## 2. Technical Specification / Findings
- `harnez skill install` today takes a git repository URL and one skill name (see `harnez skill --help`).
- Aliases map a short name to a repository URL. Store them in the user registry or spec config, not as Go
  defaults (repo rule: configurable values live in `spec/`).
- The alias must resolve before any clone, keep the commit pin and the copy-only skill directory rule, and
  never run repository code.
- Open question: where the alias map lives (user config under `~/.config/harnez`, or the external skill
  registry from 631).

## 3. Implementation & Verification Plan
- Add the alias lookup to `harnez skill install` and a way to add an alias (owner to confirm the surface).
- Test: `install egm` and `install <url>` produce the same installed skill and pin; unknown alias gives a clear error.
- Verify live: `make install`, then `harnez skill install egm`, then `harnez skill list`.
