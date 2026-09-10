# 302 — Research plugin systems for pi agents, DeepSeek Harness, and Prime Agent self-modification

**Status**: Open
**Priority**: P3 (Low)
**Severity**: N/A (research)
**Category**: Documentation
**Related**: [Research study](../docs/studies/2026-09-10-agent-harness-plugin-systems-and-self-modification.md), [Pi extensions](https://pi.dev/docs/latest/extensions), [DeepSeek Harness architecture](https://deepseek-harness.github.io/deepseek-harness/en/reference/), [Prime Agent repository](https://github.com/PrimeIntellect-ai/prime-agent)

---

## 1. Problem & Motivation

Agent harnesses are exposing extension surfaces broad enough to change tools,
prompts, orchestration, context, and runtime behavior. We need a current,
source-backed comparison of **pi agents**, **DeepSeek Harness**, and **Prime
Agent** to understand which mechanisms could support an agent improving or
modifying its own harness over time. The result must separate documented
capabilities from conclusions inferred from those capabilities; a plugin system
alone does not prove safe or autonomous self-modification.

This is exploratory research. It should produce a study or design note and,
only after review, any implementation follow-up tickets. It must not modify
harnez code as part of this ticket.

## 2. Technical Specification / Findings

Research the three projects using current primary sources, preferably official
documentation and source repositories. At minimum, cover:

- **Pi agents**: TypeScript extensions, global and project-local discovery,
  hot reload, lifecycle event interception, custom tools and commands, prompt
  and context mutation, package distribution, and the full-permissions trust
  model. The official extension documentation says extensions can modify tool
  calls and system-prompt construction, and that `/reload` can hot-reload
  discovered extensions.
- **DeepSeek Harness**: the Cordis plugin kernel, plugin mounting/unmounting,
  dependency management, profiles and ordered bundles, configuration patches,
  live patch reload where supported, and the fact that models, tools, skills,
  sessions, storage, loops, scheduling, and UI are plugin capabilities. Check
  whether the docs describe runtime replacement, persistence, rollback, or
  validation; do not infer those from composability alone.
- **Prime Agent**: its relationship to Pi where relevant, extension discovery
  and reload, custom tools, lifecycle hooks, prompt/context and compaction
  customization, skills, packages, SDK resource loaders, and its documented
  “self-improving” or self-modifiable harness claims. Verify the current
  repository rather than assuming upstream Pi behavior is identical.

For each project, distinguish documented capability, plausible mechanism, and
unsubstantiated claim or uncertainty. A plausible mechanism might be an agent
writing an extension file and reloading it; that does not establish safe
autonomous promotion, regression testing, approval, rollback, or persistence.

Compare extension granularity, discovery and installation, runtime activation,
what an extension can alter, state and persistence, observability, trust and
security boundaries, and recovery after a bad change. Explain the minimum loop
needed for self-modification: propose or write a change, load it, evaluate it,
decide whether to retain it, and recover if it fails. Identify which steps are
built in and which require an external harness or workflow.

Use stable source links in the eventual research document and record the date
accessed, project/version or commit where available, and uncertainties caused by
developer-preview status or rapidly changing repositories. Treat search-result
snippets, community posts, and papers as leads or context; do not present them
as proof of implementation when official sources disagree or are silent.

## 3. Implementation & Verification Plan

- Search for duplicate or adjacent research before starting the study.
- Read the three projects’ current official extension/plugin documentation and
  relevant source examples; capture direct URLs and version/commit context.
- Write a focused study under `docs/studies/` with a comparison and explicit
  evidence, inference, and uncertainty labels.
- Include a recommendation for what harnez should learn from these systems, if
  warranted, while keeping implementation proposals separate from findings.
- Verify links, dates, and claims against the cited sources; note any source
  that is unavailable, unstable, or only community-maintained.

The deliverable is documentation and research only. Do not implement a plugin
system, self-modification loop, agent integration, or unrelated documentation
cleanup under this ticket. A later ticket may address a concrete design after
the research is reviewed.
