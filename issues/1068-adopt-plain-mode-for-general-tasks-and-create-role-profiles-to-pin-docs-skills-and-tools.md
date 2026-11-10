# 1068 — Adopt plain mode for general tasks and create role profiles to pin docs, skills, and tools

**Status**: Backlog — grooming 1083 (2026-11-03): active list focused on taia support, P1s and open bugs; promote when needed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [1027](1027-agent-context-modes-plain-no-skills-role-skills-for-pi-and-small-window-models.md), [1057](1057-epic-plain-agent-runs-and-temporary-model-selection.md), [926](926-support-installing-custom-repository-provided-skills-into-agent-environments.md), [1034](1034-local-model-runs-attach-all-input-as-files-and-give-tight-role-guidance-so-the-agent-reads-nothing-itself.md)

---

## 1. Problem & Motivation

Issue 1027 introduced selectable agent context modes (`default`, `no-skills`, and `plain`) in `spec/agent.yaml` and the subagent runner (`--context-mode`). While `plain` mode successfully strips all context files, skills, MCP servers, and restricts tool exposure to the core coding set `[read, edit, write, grep, find, bash]`, it is currently a fixed, one-size-fits-all definition.

In practice, general agent tasks require distinct contextual and tooling configurations depending on their specific role:
- **Reviewers**: Do not need write/edit tools, and benefit from read-only tools plus review-specific guidance docs.
- **Web Researchers / Explorers**: Require web search or browser inspection tools, but do not require code editing or repository-wide skills.
- **Coders / Mechanical Implementers**: Need exact file mutation and build tools, but do not need extraneous orchestrator or communication skills.

Currently, there is no declarative way for users (globally in `~/.config/harnez/`) or repositories (locally in `.harnez/repo.yaml` or `.harnez/profiles/`) to define **role profiles** that compose context modes, pinning specific subsets of documentation, skills, and tools to specific agent roles.

## 2. Technical Specification & Design

### 2.1. Schema and storage

The examples below define the proposed configuration contract, not an assertion that these keys already work. Use the same `role_profiles` mapping in the embedded `spec/agent.yaml`, user `~/.config/harnez/local.yaml`, and repository `.harnez/repo.yaml`. A mapping key is the canonical role; standalone files instead require `role`. Do not also accept an ambiguous `profiles` alias.

Global inline example (additive to other `local.yaml` settings):

```yaml
# ~/.config/harnez/local.yaml
role_profiles:
  reviewer:
    context_mode: plain
    docs: [docs/ReviewChecklist.md]
    skills: []
    tools: [read, grep, find]
  developer:
    context_mode: plain
    docs: []
    skills: []
    tools: [read, edit, write, grep, find, bash]
```

Global standalone example (overlays the inline reviewer above):

```yaml
# ~/.config/harnez/profiles/reviewer.yaml
version: 1
role: reviewer
docs: [docs/PersonalReview.md]
# Omitted context_mode, skills, and tools inherit from lower layers.
```

Repository inline example (retain other existing repo settings):

```yaml
# .harnez/repo.yaml
version: 1
role_profiles:
  reviewer:
    context_mode: plain
    docs: [docs/ReviewLearnings.md]
    skills: []
    tools: [read, grep, find]
```

Repository standalone example (overlays the repository inline reviewer):

```yaml
# .harnez/profiles/reviewer.yaml
version: 1
role: reviewer
docs: [docs/ReviewLearnings.md, docs/ReviewChecklist.md]
skills: [review-checklist] # Example installed skill ID; must exist at launch.
tools: [read, grep, find]
```

- Profile fields: optional `context_mode` enum (`default`, `no-skills`, `plain`) and optional `docs`, `skills`, `tools` arrays of unique, nonempty strings. Reject nulls, unknown fields, duplicate YAML keys, and invalid types; an empty profile object is a valid no-op overlay.
- Standalone files require `version: 1` and `role`; filename `<role>.yaml` must match `role`. Read only direct `*.yaml` children; no recursion or filename aliases. Missing optional files/directories are normal; unreadable or malformed discovered configuration is an error naming its path and field.
- Global root is `$XDG_CONFIG_HOME/harnez` when set, otherwise `~/.config/harnez`. Global relative doc paths resolve against that root, including docs declared in `profiles/`; repository relative paths resolve against the selected project root, never the process working directory or `.harnez/profiles/`. Absolute paths and leading `~/` are allowed; no environment interpolation, globs, URLs, or implicit topic lookup in doc entries. Carry each winning field's source/base directory through resolution.
- Docs are existing readable files, including explicitly named topic-card files; skill entries are exact installed skill IDs. Resolve skills through the existing runner's skill registry without installing or downloading anything. Validate selected docs/skills only after merging, so a replaced lower-layer path need not exist on this machine.
- Extend the agent, user-local, and repo JSON schemas with a shared profile definition and add a standalone-profile schema in `spec/schemas/`. Preserve existing config fields and compatibility when `role_profiles` is absent. Builtin role defaults, tool identifiers/provider mappings, and configurable values belong in `spec/`, never duplicated as Go defaults.

### 2.2. Exact resolution order

First normalize/select the role using the existing role aliases and default-role rules; explicit `--role` selects the lookup key, rather than overriding a field inside a profile. Profiles cannot introduce new execution roles or grant orchestration privileges. Reject unknown CLI roles or profile role keys (including `searcher`/`explorer` unless already registered in the spec); list supported roles instead of falling back to developer.

For that role, merge fields from lowest to highest precedence:

1. Embedded `spec/agent.yaml` builtin role profile.
2. User `local.yaml: role_profiles.<role>`.
3. User `profiles/<role>.yaml`.
4. Repository `.harnez/repo.yaml: role_profiles.<role>`.
5. Repository `.harnez/profiles/<role>.yaml`.
6. Explicit CLI field overrides: `--context-mode` and `--tools`.

- Resolve each field independently: omission inherits, a supplied scalar replaces, and a supplied array replaces the entire inherited array. No implicit union, append, intersection, or cross-role inheritance. An explicitly empty array clears the inherited list. Track CLI flag presence, so a parser default never overrides a profile.
- `--tools` accepts a comma-separated exact list; repeated occurrences concatenate into one CLI list, then deduplicate in first-seen order. `--tools=` means zero tools; reject empty interior entries. This CLI list replaces the resolved profile list. No new docs/skills CLI flags are required in this ticket.
- After merging, any still-omitted context mode uses the existing spec-defined default; omitted tools/docs/skills use the selected mode's baseline. Explicit lists, including `[]`, replace those baseline lists. Changing only `--context-mode` does not discard profile docs/skills/tools. An unprofiled registered role retains its existing spec-defined behavior.
- Example using the four files above: reviewer gets `plain`, two repository docs, `review-checklist`, and `[read, grep, find]`. Adding `--tools read,grep` changes only tools to `[read, grep]`. Adding `--context-mode default` changes only the baseline mode; it does not restore ambient tools over the explicit whitelist.
- Ship builtin plain profiles for general developer, reviewer, and advisor tasks; developer gets the core coding tool set, reviewer/advisor get `[read, grep, find]`, and all three get empty docs/skills unless explicitly pinned. Preserve orchestrator defaults and lifecycle authority. Keep direct mode definitions intact; role profiles are applied above them.

### 2.3. Tool and context edge cases

- `tools` is an exact exposure whitelist, replacing the baseline rather than adding to it. To keep baseline tools and add another, enumerate the full desired set. Do not expose extra tools because a pinned skill requests them. An empty whitelist must launch with zero tools or fail explicitly if the provider cannot enforce it.
- Validate the final whitelist against the spec-defined logical tool registry and the chosen provider's capabilities before spawning a process. Unknown tools, missing tool executables/extensions, disabled MCP dependencies, or unavailable provider mappings fail with the role, source, tool, and remediation; never silently drop a tool or broaden permissions. A tool available only via MCP remains unavailable in plain mode unless a future explicit capability enables it; a tool name alone must not enable an ambient MCP server.
- In plain mode, explicitly pinned docs and skills are intentional additions to the otherwise empty context. Only those pins may be loaded; suppress ambient context, skill discovery, and MCP as before. In no-skills mode, explicit skill pins similarly opt in only those skills. An explicit docs/skills list in default mode filters that category to the pins; an omitted list uses its baseline. Pinned content cannot implicitly enable another skill or tool.
- Missing/unreadable final doc pins or uninstalled skill IDs fail before launch with the declaring file and entry. Do not silently continue with less guidance. Invalid lower-layer schema is still an error even when its fields would be overridden; resource availability checks apply only to winning values.
- Read-only reviewer defaults omit `bash` as well as `write`/`edit`: shell access can mutate files. Tool exposure is not an OS sandbox; existing authorization restrictions still apply. Unsupported enforcement on a provider must return an actionable capability error rather than claiming the profile was honored.

### 2.4. CLI and provider contract

- Resolve once in the shared start path, then pass the same effective mode, pins, exact whitelist, and field provenance to the provider adapter, including `piRunArgs`. Resolve profiles even when `--context-mode` is explicit; that flag overrides one field only.
- Pi, Claude, and AGY adapters must either enforce the resolved configuration or reject unsupported capabilities before starting the agent. Do not approximate a whitelist with prompt-only instructions. Preserve existing lifecycle/permission checks; role selection and profile data cannot bypass them.
- Make the effective profile and winning field sources available in existing launch diagnostics without dumping pinned document contents. No changes to `apply`/`init` scope, automatic profile installation, skill installation, or custom execution-role registration are required.

## 3. Goal & Acceptance Plan

/goal Implement declarative role profiles (global and repo-local) to pin docs, skills, and tools to agent roles on top of plain context modes, verified with configuration resolution tests and subagent invocation unit tests, or stop and report when blocked on user input or denied permission.

### M1 — Schema and resolution engine

- Add shared schema definitions, standalone schema, builtin profiles, and a single resolution engine in the appropriate existing policy/subagent package. Return effective fields with source provenance; retain field-presence information through parsing.
- Add schema fixtures for all four examples (with temporary docs and an installed-skill fixture), absent config, empty lists, partial profiles, invalid modes/types/nulls/keys, duplicate keys, unsupported roles, and standalone role/filename/version mismatches.
- Add table-driven resolution tests covering all six precedence layers, both inline-versus-file ties, per-field inheritance, replacement rather than union, CLI presence versus defaults, role isolation, unprofiled-role compatibility, XDG roots, doc path bases, and missing versus malformed configuration. Assert expected literal values and provenance independently of the resolver.
- **M1 exit:** validated fixtures and resolution tests pass; resolution does not spawn agents, install resources, or modify user/repo configuration. The resolved examples above have explicit assertions. M1 is not completion of the feature without M2.

### M2 — Driver/CLI plumbing and verification tests

- Wire `agent start`, `--role`, `--context-mode`, and `--tools` into resolution and each provider adapter. Add preflight validation and effective-profile diagnostics; document precedence, list clearing, and plain-mode pin exceptions in user-facing CLI/configuration docs.
- Add invocation tests with isolated HOME/XDG/project roots and captured provider argv/config/context. Verify CLI context-only overrides retain pins, tools override exactly (including repeated flags and `--tools=`), no ambient docs/skills/MCP leak into plain runs, and missing resources/unknown tools/unsupported provider capabilities fail before process start.
- For Pi, Claude, and AGY, test either exact enforcement or the explicit unsupported-capability error. Verify reviewer defaults expose no mutation tools or shell, and a selected pinned skill cannot expand the whitelist. Preserve regression assertions for unprofiled roles and all three context modes.
- Run a real installed-binary reviewer canary through a supported provider with a temporary repo profile and pinned review doc: confirm plain context, the pinned doc, and exactly `[read, grep, find]`. Record command, provider, and observed effective tools/context in this ticket; use a host orchestrator for live agent dispatch when implementing as a leaf worker. Mock invocation tests alone do not satisfy this gate.
- **M2 exit:** required repository checks pass, `make install` succeeds, the live canary confirms enforcement, user docs match the implementation, and review verifies test assertions and compatibility. Record evidence and clean up temporary configs/processes.

**Definition of done:** M1 and M2 exit criteria are met; global and repo profiles demonstrably resolve with the specified precedence, explicit pins and exact tools reach the runner, invalid or unsupported configurations fail before launch, and the implementation is reviewed and committed with verification evidence linked here. Keep this ticket Open during advisory refinement; close it only after implementation and verification, then sync the tracker.

**Exit clause:** Preserve the `/goal` above. If blocked on user input or denied permission, stop and report the decision/permission needed, attempted checks, and exact resume point here. A hard provider/OS blocker or no-progress loop also requires a failure report and clearing any active execution goal; do not silently weaken the whitelist, drop pins, or mark an unverified milestone complete.

## Decision

- Standalone files override inline profiles within each scope; arrays replace instead of union. This makes one winning source explain each field and prevents inherited tools from silently expanding a restricted role.
- This refinement defines a proposed contract without code exploration; the implementation must verify existing config/driver entry points before editing them while retaining these observable semantics.

## Audit 2026-11-11

Jules audit (PR GH#164, report `reports/jules/1068.md`, checked against e8b0c7ce, 2 of 2 references held;
the absence search for role profiles repeated by the host, also empty): still valid, M1 not started.
`internal/subagent/agentspec.go:39` has only `ContextModes`; no profile schema, resolution or tests.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-1068 | 3700776250448050743 | github #164 (closed) | done: still valid, 2/2 refs hold |
