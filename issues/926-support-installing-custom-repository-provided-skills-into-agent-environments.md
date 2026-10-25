# 926 — Support installing custom repository-provided skills into agent environments

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 

---

## 1. Problem & Motivation
When an individual repository provides an external or domain-specific custom skill (for example, repository-managed QA or domain skills like `skills/neus-study/SKILL.md` in `neus`), there is currently no standardized CLI mechanism in `harnez` to install and register that skill for active agent harnesses.

Developers and orchestrators currently have to manually discover, copy, or symlink skill directories into agent-specific configuration paths (such as `~/.gemini/skills/` or equivalent agent skill paths). Using `harnez` as a unified skill installer would allow repositories to cleanly ship their own skills and have `harnez` manage their installation and discovery across supported agent environments.

## 2. Technical Specification / Findings
- Provide a `harnez` command to install custom skills:
  - e.g. `harnez skill install [path]` or `harnez install skill <path>` (defaulting to scanning `./skills/*/SKILL.md`).
  - Target known agent configuration directories (e.g. `~/.gemini/skills/`, Claude, Codex, etc.).
- Support installing individual skills or all skills exposed by the current repository.
- Support link mode (symlink for active local development) vs copy mode.
- Validate skill metadata (YAML frontmatter with `name` and `description`) prior to installation.

## 3. Implementation & Verification Plan
- Details and command surface to be refined within the harnez development flow.
- Verify that running the installer registers a repository skill so that target agent runtimes immediately discover the new skill.
