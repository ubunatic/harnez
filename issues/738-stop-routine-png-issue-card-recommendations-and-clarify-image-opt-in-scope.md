# 738 — Stop routine PNG issue-card recommendations and clarify image opt-in scope

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: #626, #546, #543

---

## 1. Problem & Motivation

Codex still executes `harnez find -d /home/uwe/projects/uman issues "push default remotes" -I`
and views `/tmp/harnez_find_issues.png`, despite the user's understanding that PNG cards
were disabled. The reported card estimated ~288 Claude ViT tokens versus ~183 text tokens;
this is not a measured Codex cost, but gives no demonstrated benefit for this small search.

/goal Make routine agent issue discovery use text and document exactly when PNG cards
remain available; verify generated rules and installed skills no longer steer agents to
images by default, or stop and report when blocked on a user decision or denied permission.

## 2. Technical Specification / Findings

- #626 disabled automatic image selection for `harnez read --auto`; explicit `-I` remains
  supported. The pause in #546 concerns the `read -I` recommendation, not a universal image ban.
- `config.yaml:617-619` still recommends `find issues -I` and `issues show -I`; these appear
  in this checkout's generated `.harnez/rules/Tools.md`.
- `docs/commands/issue.md` and the installed `~/.codex/skills/issue/SKILL.md` include `-I`
  in the duplicate-search workflow. Review, roadmap, and proposed discovery guidance
  also recommend issue PNG cards. Agents following that guidance explicitly opt into images.
- `cmd/harnez/find.go` and `cmd/harnez/issues.go` still expose `-I/--image`, default false.
  Explicit image reads and existing PNG documentation links are further image entry points.

## 3. Implementation & Verification Plan

- Recheck live code, recent history, and #626/#546 before implementation.
- Remove routine issue-card recommendations from canonical rules and skill sources;
  sync generated/installed copies through the appropriate init/apply/skill workflows.
- State whether explicit images require a user request or may be selected for a proven
  benefit; do not assume the user requested removal of the renderer itself.
- Verify plain issue search returns text and generated guidance uses that path.
  Cover guidance regressions and verify any retained explicit image path separately.
