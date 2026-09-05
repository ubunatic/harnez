# 249 — Copied Practice Docs Retain Dangling and Inapplicable Dependencies

**Status**: In Progress — begin copied-doc dependency validation
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [[018-mark-bundled-docs-in-frontmatter]],
[[041-sibling-projects-real-world-shape-and-test-suite-extensions]],
[[130-instruction-distribution-audit-followups]], `docs/practices/AgenticLoop.md`,
`docs/practices/DeploymentTransparency.md`, `docs/practices/IssueTracking.md`

---

## 1. Problem & Motivation

A live audit in the sibling `emojig` repository found that `harnez init` can install a copyable
practice doc whose text is byte-for-byte canonical while leaving its assumptions and references
invalid in the receiving project.

`emojig/docs/AgenticLoop.md` currently references:

- `docs/practices/DeploymentTransparency.md`, which is not installed in emojig;
- two concrete harnez/smarthome case studies under `docs/studies/`, which are not installed;
- `docs/feedback/` and `docs/studies/` as durable workflow destinations whose availability was not
  guaranteed when the doc was first inspected.

The deployment-transparency invariant is also inapplicable to emojig: its project instructions
explicitly reject the remote-host/daemon architecture that the invariant governs. Copying the
section verbatim therefore creates both a dangling documentation edge and irrelevant normative
boilerplate. An agent cannot tell whether to install the missing dependency, ignore the rule, or
change the receiving project's architecture to satisfy it.

This is a distribution-model gap, not a request to add remote deployment machinery or harnez's
private case-study corpus to emojig.

## 2. Reproduction & Evidence

Observed after `harnez init` updated emojig during an active session:

1. `docs/AgenticLoop.md` matches harnez's canonical copy.
2. Its `IssueTracking.md` §5 cross-reference now resolves because init installed an updated
   `IssueTracking.md`; this demonstrates that transitive references can become valid accidentally
   through another selected doc.
3. These referenced files remain absent:
   - `docs/practices/DeploymentTransparency.md`
   - `docs/studies/2026-08-30-usage-panel-integration-and-the-agy-collector-cliff.md`
   - `docs/studies/2026-09-04-three-days-to-a-public-release.md`
4. The copied `AgenticLoop.md` lacks the load-bearing `<!-- harnez:bundled -->` marker that the
   copied `IssueTracking.md` carries. That marker-coverage defect is already tracked by issue 018
   and must not be duplicated in this ticket.

The presence of the `docs/studies/` and `docs/feedback/` directories may change as a project is
initialized or worked on; directory existence alone does not repair missing referenced files or
make the prescribed workflow relevant.

## 3. Root Design Gap

Copyable docs are currently treated as independent byte payloads, but their content forms a graph:

- hard dependencies: another doc is required to understand or follow a rule;
- illustrative references: a missing case study should not invalidate the rule;
- capability-scoped sections: remote deployment guidance only applies to projects with that
  architecture;
- destination assumptions: workflow text names directories that may need creation, fallback
  locations, or explicit opt-in.

The install/config model does not appear to express these distinctions. As a result, a source doc
can be internally correct in harnez and still be incomplete or misleading when copied alone.

## 4. Exploration Scope

Determine and canary the smallest maintainable contract. Candidate directions include:

1. Declare hard doc dependencies in `config.yaml` and have `init --docs <name>` resolve them
   transitively.
2. Make copyable docs self-contained: replace repository-relative illustrative links with stable
   upstream links or short inline context, while keeping true dependencies explicit.
3. Split architecture-specific invariants (such as deployment transparency) out of the generic
   Agentic Loop doc and compose them only for projects selecting that capability.
4. Add capability/profile-aware section pruning during install, but only if a canary proves this
   does not create hard-to-review generated variants or duplicate source-of-truth prose.
5. Add a docs-graph validator that rejects copyable docs with undeclared relative links and
   unmodeled directory assumptions.

Do not select an approach solely to repair emojig. Audit all copyable docs for the same class of
dependency and applicability drift first.

## 5. Constraints

- Preserve the load-bearing `apply` (global installation) versus `init` (project-local materialization)
  boundary in `docs/CLIDesign.md`.
- Do not silently copy every study or practice doc into every project.
- Do not require projects to adopt irrelevant architectures merely because a generic workflow doc
  mentions them.
- Keep one canonical source for shared prose; avoid forks of `AgenticLoop.md` per project.
- Missing illustrative material should degrade clearly, while missing normative dependencies
  should be prevented or installed explicitly.
- Keep issue 018 responsible for adding/enforcing `<!-- harnez:bundled -->` marker coverage.

## 6. Acceptance Criteria

- [ ] A canary builds the dependency/applicability graph for all docs installable by `harnez init
  --docs`, classifying hard, illustrative, capability-scoped, and destination references.
- [ ] Installing Agentic Loop guidance into an emojig-shaped fixture produces no broken relative
  doc links and no unexplained references to unavailable case studies.
- [ ] Deployment-specific normative guidance is either installed with an explicit selected
  capability or excluded/reframed for a project that declares remote hosts and daemons out of
  scope.
- [ ] The chosen behavior is deterministic and idempotent and does not copy the entire harnez docs
  tree as an implicit dependency closure.
- [ ] Tests cover transitive hard dependencies, missing illustrative references, and an
  architecture-inapplicable section.
- [ ] `harnez status` or a dedicated validator reports broken/undeclared copied-doc dependencies
  with the source doc, reference, and remediation.
- [ ] Documentation states what a copyable doc may assume about sibling docs, project directories,
  and optional capabilities.
- [ ] Issue 018 is referenced for the separate AgenticLoop marker gap rather than reimplemented
  here.

## 7. Verification Guidance

Use a temporary git repository shaped like emojig rather than mutating the sibling repo during
tests. Run the real `init --docs agentic-loop` path, inspect every local relative Markdown link,
and assert the expected capability-scoped content. Also run the validator over harnez's complete
copyable-doc catalog so a new dangling edge cannot enter unnoticed.
