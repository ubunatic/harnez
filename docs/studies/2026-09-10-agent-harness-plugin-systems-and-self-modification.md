<!-- harnez:topic: Pi, DeepSeek Harness, and Prime Agent plugin systems and self-modification -->

# Agent Harness Plugin Systems and Self-Modification

**Scope**: Current public extension and harness mechanisms in Pi, DeepSeek Harness, and Prime Agent, and what they make possible for controlled self-modification.

**Accessed**: 2026-09-10

## Finding

All three systems expose an editable capability layer around a more stable
execution core, but they do so at different levels:

- **Pi** loads trusted TypeScript extensions that can add tools, commands, UI,
  event handlers, prompt changes, and persistent session entries.
- **DeepSeek Harness** makes nearly every runtime capability a Cordis plugin and
  composes plugins through ordered bundles and configuration patches.
- **Prime Agent** makes the harness programmable through a persistent Python
  environment, executable skills, and durable supplemental harness state that
  `/refine` can update with snapshots for rollback.

These mechanisms enable self-modification as an engineering loop, but they do
not by themselves provide safe autonomous evolution. A complete loop still
needs proposal, loading, evaluation, promotion, and recovery rules.

## Evidence conventions

- **Documented** means the linked project documentation or source explicitly
  describes the behavior.
- **Inference** means a reasonable use of a documented mechanism, such as an
  agent writing an extension and asking the host to reload it.
- **Unknown** means the reviewed sources do not establish the behavior.

## Pi

### Documented capability

Pi extensions are TypeScript modules loaded from a factory receiving an
`ExtensionAPI`. An extension can subscribe to lifecycle events, register tools,
commands, shortcuts, flags, and custom UI, and perform asynchronous startup
work. The extension API also exposes the current prompt inputs, loaded context
files, skills, and tool descriptions to prompt handlers.

Pi discovers extensions from global paths such as
`~/.pi/agent/extensions/`, project paths such as `.pi/extensions/`, explicit
paths, and installed npm or Git packages. The documentation describes reload
support for extensions and other runtime resources.

Source: [Pi extensions documentation](https://pi.dev/docs/latest/extensions)

### Self-modification mechanism

**Inference**: an agent with file and shell access can create or edit an
extension, install a package, and invoke a reload command or restart. The new
extension can alter tool availability, intercept tool calls, inject context,
modify the system prompt for a turn, or add an orchestration command. The
`pi-agents` package shows how a distributed extension can add multi-agent
workflow behavior on top of this host mechanism.

Source: [`pi-agents` package](https://pi.dev/packages/pi-agents?type=extension)

Pi therefore supplies a strong **code extension surface**, with a relatively
small built-in promotion protocol. The extension can change behavior, but the
reviewed documentation does not establish automatic regression testing,
approval, staged rollout, or rollback for an agent-authored extension.

### Boundary and risk

Pi explicitly warns that extensions execute with full system permissions and
can run arbitrary code. This makes the extension directory a trust boundary.
Project-local extensions are particularly significant: they allow repository
content to alter the agent when the project is trusted and the local scope is
enabled.

## DeepSeek Harness

### Documented capability

DeepSeek Harness is built on the Cordis plugin system. Plugins contribute
services, typed events, and reversible effects. The architecture documentation
states that the model adapter, tool registry, session log, agent loop, storage,
sandbox and approval policy, settings, credentials, telemetry, and UI are
plugin-provided capabilities.

There is no privileged product core that must be patched. A plugin is mounted
beside other plugins, and its registrations unwind when it is unloaded.

Profiles compose ordered bundles. A bundle is an installable package containing
plugin code and a patch layer. A profile has an ordered bundle list and its own
`cordis.patch.yml`; home-level and command-line patch layers are applied later.
Later layers can replace a row by ID or insert a new row. Custom profiles and
the shipped web profile support live patch reload; one-shot and stdio profiles
apply their layers at startup.

Sources: [Harness architecture](https://deepseek-harness.github.io/deepseek-harness/en/reference/), [plugin packaging and installation](https://deepseek-harness.github.io/deepseek-harness/en/develop/basic/publish), [official overview](https://www.deepseek.com/harness/en/)

### Self-modification mechanism

**Documented**: an operator or tool can install a bundle, alter a profile or
patch layer, inspect the composed tree, and reload it where the selected
profile supports live reload. The system is explicitly designed so capability
selection and replacement happen through composition rather than source edits.

**Inference**: an agent with suitable filesystem and package-management access
could author a bundle or patch, install it into a profile, inspect the resulting
configuration, and reload it. Since the agent loop itself is a plugin, this
surface can reach deeper runtime behavior than a tool-only extension system.

The reviewed sources document mounting, replacement, unload behavior, and
configuration layering. They do not establish that the harness automatically
tests an agent-authored plugin, approves it, snapshots every runtime change, or
rolls back a failed live change. Developer-preview status also means plugin
APIs and package conventions may change.

### Boundary and risk

The plugin kernel provides lifecycle structure and reversible registration
effects, but this is not the same as a complete safety policy. Installed plugin
code and patch files remain executable configuration. A self-modifying workflow
needs explicit permission, validation, and recovery around package installation
and live reload.

## Prime Agent

### Documented capability

Prime Agent describes itself as a self-improving RLM harness. Its Recursive
Language Model runtime gives the model a persistent Python control environment;
subagents are callable through `rlm(...)`, while provider calls, session
persistence, child lifecycles, scheduling, and safety policy remain in the
TypeScript host.

Its Continual Harness stores supplemental prompts, memories, skill descriptions,
and reusable subagent specifications as durable state. `/refine` reviews the
current trajectory and can apply small evidence-backed updates. The documented
design keeps the base system prompt immutable and records before/after
snapshots for rollback. Skills are importable Python packages, and the skill
creator can turn recurring workflows into project or personal skills.

Sources: [Prime Agent repository and README](https://github.com/PrimeIntellect-ai/prime-agent), [RLM programming model](https://github.com/PrimeIntellect-ai/prime-agent/blob/main/packages/coding-agent/docs/rlm.md), [RLM runtime and harness state](https://github.com/PrimeIntellect-ai/prime-agent/blob/main/packages/coding-agent/docs/rlm-runtime.md)

### Self-modification mechanism

**Documented**: Prime Agent can refine supplemental harness state, create or
use executable skills, persist the resulting state across sessions, and restore
recorded snapshots. This is a direct self-improvement path for prompts,
memories, skills, and subagent specifications while leaving the immutable base
prompt unchanged.

**Inference**: the persistent Python environment also gives an agent a
programmatic route to author files, run tests, create skills, and coordinate a
proposal/evaluation cycle. That makes Prime Agent more than a static plugin
loader: the model-facing control surface itself is programmable.

Prime Agent is therefore best described as a **self-modifying harness state and
skill system**, rather than primarily as a generic plugin registry. The
reviewed sources do not show that `/refine` automatically rewrites arbitrary
host code, changes provider implementation, or promotes executable skills
without review. The repository warns that model-generated Python and project
commands run with the user's permissions and that the kernel is not a security
sandbox.

## Comparison

- **Extension granularity**: Pi centers on host extensions; DeepSeek Harness
  decomposes the runtime into plugins; Prime Agent exposes programmable RLM
  state and skills alongside a TypeScript host.
- **Activation**: Pi uses discovery, explicit paths, packages, and reload;
  DeepSeek Harness uses profile bundles and patch layers, including live reload
  for selected profiles; Prime Agent uses persistent session state, skills, and
  refinement operations.
- **What can change**: Pi can change tools, prompts, events, UI, and workflow
  commands; DeepSeek Harness can replace most runtime components; Prime Agent
  can change supplemental prompts, memories, skills, and subagent definitions.
- **Persistence**: Pi extensions persist as files or installed packages and
  session entries can persist state; DeepSeek Harness persists profiles,
  bundles, and patch files; Prime Agent persists harness state and refinement
  snapshots in session or global storage.
- **Recovery**: Pi recovery is largely external; DeepSeek Harness has unloadable
  registrations and layer removal, but the sources do not promise full state
  rollback; Prime Agent explicitly documents before/after snapshots for harness
  refinements.
- **Trust boundary**: all three can execute trusted code with substantial user
  permissions. Prime Agent explicitly says its Python kernel is not a sandbox;
  Pi explicitly gives extensions full system permissions; DeepSeek Harness
  plugin packages and patch layers must likewise be treated as trusted code and
  configuration.

## Minimum self-modification loop

A useful implementation model is:

1. **Propose** a bounded change from an observed failure or improvement goal.
2. **Write** it as an extension, bundle, patch, skill, or supplemental state
   update.
3. **Load** it in a disposable or isolated session when possible.
4. **Evaluate** it against a targeted task and regression checks.
5. **Promote** it only after an explicit decision and record the version or
   snapshot.
6. **Recover** by unloading, removing the layer, restoring the snapshot, or
   returning to a known-good package.

The projects cover these steps unevenly:

- Pi documents writing and loading extensions, but the proposal, evaluation,
  promotion, and rollback loop must be supplied by an extension or external
  workflow.
- DeepSeek Harness documents package/profile composition, patch ordering,
  inspection, and some live reload behavior. Evaluation and promotion policy
  remain an external responsibility.
- Prime Agent documents refinement, durable state, evidence-backed updates, and
  rollback snapshots. It still needs task-specific regression checks and a
  trust policy for executable skills and Python changes.

## Implications for Harnez

The useful shared idea is a **bounded, reviewable capability layer**. If Harnez
later supports agent self-modification, it should represent each change as an
explicit artifact with a scope, source, validation result, activation method,
and recovery record. The artifact could be a managed skill, command, resource,
or project instruction, but the activation and rollback metadata should be
separate from the content being changed.

The strongest lessons are:

- Keep a stable core and make changes additive or layered where possible.
- Make discovery and activation inspectable before executing new code.
- Separate project-local changes from global changes.
- Record the exact source, version, patch, or snapshot used for activation.
- Require targeted verification before promotion.
- Treat executable extensions, skills, and patches as trusted code.
- Make removal or rollback a first-class operation rather than relying on a
  process restart.

These are design implications, not a proposal to implement self-modification in
Harnez under this study.

## Astra assessment for Harnez

The previous Astra advisor assessed that Harnez would benefit from a limited
plugin concept, but that a general runtime plugin system would currently add
more risk than value. The useful first step is a declarative **capability
bundle** layer over the existing `config.yaml` → `apply`/`init` pipeline.

### Expected value

The value is moderate to high for packaging and distribution as the existing
resource mechanism grows. A capability bundle could package a skill, commands,
supporting resources, documentation, and optional configuration; distribute
harness-specific adapters; enable or disable optional capabilities; and make
source, target, version, and validation provenance inspectable.

Potential use cases include:

- Claude hooks, AGY shims, Pi extensions, or Prime skills distributed as named
  capabilities.
- Project-scoped bundles installed through `init` and global bundles installed
  through `apply`.
- Future telemetry collectors or integration adapters that use existing Harnez
  extension points.

The value is low for replacing Harnez's internal architecture. `apply`, `init`,
`diff`, `status`, and `clean` already provide the central lifecycle and should
remain authoritative.

### Minimal design

The first version should be declarative and reuse existing skills, resources,
hooks, docs, and target expansion code rather than introduce a second
installer. A manifest could live under `plugins/<name>/plugin.yaml`, with
`config.yaml` enabling selected manifests:

```yaml
plugins:
  - name: docup
    version: "1"
    scope: global
    skills:
      - docup
    resources:
      - source: docs/commands/Docup.md
        target: ...
```

The manifest should record:

- a stable name and version;
- the supported Harnez schema or API version;
- owned artifacts and target scope (`global`, `project`, or both);
- required validations;
- source provenance; and
- explicit enablement.

A capability bundle should expand into existing managed artifacts. Its ownership
must remain visible so collisions, diffs, status, cleanup, and migrations are
deterministic. The core commands must continue to own installation behavior and
scope precedence.

### Boundaries

The initial design should exclude:

- dynamic Go loading or arbitrary in-process plugins;
- plugins replacing `apply` or `init` semantics;
- changes to core configuration precedence or security policy;
- automatic network installation;
- autonomous self-modification and promotion;
- silent creation of global directories or project files; and
- plugin-owned credentials, permissions, or release behavior without dedicated
  opt-in.

Executable plugins, if ever added, should use a versioned subprocess protocol
with explicit capabilities, approval before installation, disposable
evaluation, recorded provenance, and rollback. Pi's full-permission extensions
and Prime Agent's unsandboxed Python kernel show why “plugin” cannot imply
“safe.” DeepSeek Harness's unloadable composition is useful, but Harnez would
also need file-level rollback and ownership records.

### Open design questions

- Is the primary goal reusable packaging, third-party extensibility, or agent
  self-modification? The MVP should target packaging.
- Should this be a new manifest format, or should existing skills and resources
  gain grouping and provenance fields first?
- Should sources be embedded Harnez assets, local paths, sibling repositories,
  or a package registry?
- Are executable plugins intended at all, or should Harnez remain declarative?
- What ownership and migration rules apply when a bundle removes or renames a
  resource?
- Should enablement be global, per harness, per project, or all three?
- What approval and rollback semantics are required before a bundle can install
  hooks or executable adapters?
- Does “plugin” add useful meaning, or is “capability bundle” a better name for
  the safe first version?

## Limits and source status

This study uses public documentation and source repositories accessed on
2026-09-10. DeepSeek Harness is presented as a developer preview, so its plugin
APIs and profile behavior may change. Pi and Prime Agent documentation are
active project documentation and may also track moving releases. Search-result
snippets, community posts, and papers were treated as leads; the findings above
use the linked project documentation or repositories for implementation claims.
The sources reviewed do not establish a universal safety model, automatic
regression suite, or autonomous approval process for self-modification.
