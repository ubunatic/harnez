# 738 — Stop routine PNG issue-card recommendations and clarify image opt-in scope

**Status**: Closed — Text-default guidance installed and reviewed; live checks and full make test-q1 passed
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

## Implementation & Verification (2026-10-07)

- Canonical rules and issue/review/roadmap/discovery workflows now use text. New PNG
  rendering requires an explicit user request; existing PNG documentation remains readable.
- Regenerated this checkout's Tools.md, ran `make install`, and applied updated skills
  to all configured agent targets. Other existing checkouts need `harnez init` to refresh
  their project rules; already-running agents may retain old instructions in context.
- Added regression assertions for generated rules and installed issue/review/roadmap skills.
  Independent native reviewer found no blockers; scoped whitespace check passed.
- Live checks: plain issue search/show and `read --auto` return text; explicit `show --image`
  still renders a PNG.
- The user's combined-tree test run exposed stale v9 telemetry expectations after #735's
  v10 bump. Apply tests now compare against a freshly created DB's schema version.
  `make test-q1` passed on the combined tree (formatting, vet, full Go suite), including
  the generated-rule and installed-skill assertions. `make install` completed again.
