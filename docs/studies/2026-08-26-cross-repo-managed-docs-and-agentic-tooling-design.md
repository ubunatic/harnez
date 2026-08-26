# Cross-Repo Managed Docs, Interactive Grounding, and Agent-First Tooling Design

> **Who this is for** — system architects, harness maintainers, and engineers building multi-repository agentic workflows who need consistent conventions across dozens of projects without crushing autonomy or eroding local domain knowledge.
>
> **Read this if** — you are designing developer tools or conventions that will be read, reconciled, and executed primarily by autonomous AI coding agents rather than humans alone.\
> **Skip this if** — you only operate within a single monolithic codebase with a single tech stack.
>
> **Takeaways**
> 1. **Ground Policy Interactively Before Automating**: Iterating through 28 real-world sibling repositories with an interactive confirmation gate proved out edge cases (umbrella workspace roots, stop-marker boundaries, paradigm-specific conventions) that speculative design would have missed.
> 2. **Product Management Discipline (Resist Mid-Flight Automation)**: When repetitive friction reveals an obvious automation opportunity, file a well-structured backlog ticket ([Issue 068](file:///home/uwe/projects/harnez/issues/068-harnez-sync-autonomous-doc-reconciliation-command.md)) rather than abandoning the active goal to build meta-tooling mid-session.
> 3. **Agent-First Tooling Design**: Every CLI command, output format, and file boundary marker must be built around how an AI agent will parse, reason over, and act upon it.
> 4. **Conservative Upstream Promotion & Local Retention**: Autonomous subagents must be strongly discouraged from over-generalizing single-project habits into universal templates. Defaulting to local retention (`<!-- harnez:stop -->`) preserves domain nuances safely.

---

## 1. The Challenge: Synchronized Conventions Across 28+ Sibling Repositories

In a multi-project agentic ecosystem, standardizing base engineering practices (TDD workflows, Makefile targets, commit discipline, canary probes, issue tracking) is essential for agent reliability. If each repository reinvents basic expectations, agents burn excessive tokens rediscovering idiosyncratic rules or hallucinating non-existent conventions.

`harnez` solves this by maintaining a single source of truth for canonical language and practice docs (`docs/lang/`, `docs/practices/`, `docs/other/`) and syncing them into sibling projects via `harnez init -d <project>`.

However, scaling this model across 28 diverse projects (spanning Go, Zig, Python/GTK4, Bash, Rust, C++, and static websites) surfaces a fundamental tension:
* **Upstream Synchronization**: How do you propagate improvements to core practices (e.g. fast-path `/fresh-sprint` workflows, canary boundary clarifications) without corrupting project-local custom rules?
* **Local Customization vs. Drift**: When a project diverges from upstream docs, how do you distinguish between *unintended stale drift* (which should be overwritten) and *deliberate project-local specialization* (which must be preserved)?

---

## 2. Grounding the Mechanics: Stop Markers & Umbrella Containers

Before attempting any bulk synchronization, we built and grounded two foundational mechanisms in `harnez`:

### 2.1 The Scan-Docs Engine & Multi-Repo Discovery (`harnez scan-docs`)
We implemented `harnez scan-docs <path>` ([Issue 062](file:///home/uwe/projects/harnez/issues/archive/062-scan-docs-across-agent-projects.md)) to inspect managed docs across one or many repositories without mutating disk state.

A critical edge case emerged during initial exploration: the workspace root `/home/uwe/projects` itself contained an umbrella `AGENTS.md` file. A naive repo detector treated `/home/uwe/projects` as a single project, failing to discover its 28 immediate child projects. We resolved this by explicitly checking whether a target directory is a parent container holding multiple child repositories with their own `AGENTS.md` / `CLAUDE.md` files.

### 2.2 The Stop-Marker Convention (`<!-- harnez:stop -->`)
When inspecting `webman/docs/Go.md`, we found genuine local notes ("Prefer single-line error handling", "Always check `r.Context().Done()`"). Overwriting the file with upstream `docs/lang/Go.md` would destroy valuable project domain knowledge, while ignoring the file would leave `webman` permanently out of sync with upstream updates.

We established the **Stop-Marker Boundary** ([Issue 060](file:///home/uwe/projects/harnez/issues/archive/060-triage-sibling-managed-docs-drift.md)):
```markdown
# Go Development Guidelines
... [Canonical Harnez-Managed Content] ...

<!-- harnez:stop -->

## Webman-Specific Runtime Conventions
- Custom handler lifecycle notes...
- MariaDB context rules...
```

The comparator in `harnez` was updated to parse `ExtractManagedDocContent`, comparing only the upstream portion before `<!-- harnez:stop -->` or `<!-- harnez:end -->`. Everything following the marker is recognized as local customization and protected across future `harnez init` syncs.

---

## 3. The Interactive Sweep: 28 Repositories, 100% Synchronization

Armed with `harnez scan-docs` and stop markers, we undertook a deliberate, step-by-step sweep across all 28 sibling projects under `/home/uwe/projects`.

Rather than executing an unvetted batch script, the agent operated under strict human-in-the-loop pairing:
1. Scan target repository (`harnez scan-docs <repo>`).
2. Classify differences (pure upstream drift vs. local conventions vs. candidates for upstream promotion).
3. Present findings and ask for explicit operator approval via interactive modal (`ask_question`).
4. Execute `harnez init -d <repo>` (and `--docs <list>` for selective stacks).
5. Verify clean state with `harnez scan-docs <repo>`.

```
Sibling Sweep Results (28 / 28 Repositories Reconciled):
├── books          [Go, Bash, Make, Canary, Spec]                ──> Clean
├── cati           [Go, Bash, Make, Canary, Spec]                ──> Clean
├── comfyconf      [Go, Bash, Make, Canary, Spec]                ──> Clean
├── conreel        [Go, Bash, Make, Canary, Spec]                ──> Clean
├── diagnose       [Go, Bash, Make, Canary, Spec]                ──> Clean
├── emojig         [Go, Bash, Make, Canary, Spec]                ──> Clean
├── fontwidth      [Go, Bash, Make, Canary, Spec]                ──> Clean
├── goha           [Go, Bash, Make, Canary, Spec]                ──> Clean
├── harnez.org     [Go, Make, Canary, Spec]                      ──> Clean
├── lmcoder        [Go, Make, Canary, Spec + Evergreen Docs]     ──> Clean (Local docs preserved)
├── loom           [Go, Bash, Make, Canary, Spec]                ──> Clean
├── mdview         [Go, Bash, Make, Canary, Spec]                ──> Clean
├── pdf-doctor     [Bash, Make, GTK4, Canary, Spec]              ──> Clean (Standardized GTK4)
├── proctop        [Go, Make, Canary, Spec]                      ──> Clean
├── psync          [Go, Make, Canary, Spec + Local README]       ──> Clean (Local docs preserved)
├── spriteview     [Bash, Make, GTK4, Canary, Spec]              ──> Clean (Standardized GTK4)
├── ubunatic.com   [Make, Canary, Spec + Traffic Sim Docs]       ──> Clean (Traffic rules protected)
├── uman           [Go, Make, Canary, Spec]                      ──> Clean
├── uzu            [Go, Bash, Make, Canary, Spec]                ──> Clean
├── venile.de      [Bash, Make, Canary, Spec, AgenticLoop]       ──> Clean
├── vimconf        [Go, Bash, Make, Canary, Spec]                ──> Clean
├── vkfusion       [Go, Bash, Make, Canary, Spec]                ──> Clean
├── voxi           [Go, Make, Canary, Spec]                      ──> Clean
├── wayreel        [Go, Bash, Make, Canary, Spec, AgenticLoop]   ──> Clean
├── webman         [Go, Make, Canary, Spec + Stop-Marker]        ──> Clean (Stop-marker preserved)
├── ziggo          [Go, Bash, Make, Zig, Canary, Spec]           ──> Clean
├── zterm          [Zig, Canary, Spec, AgenticLoop]              ──> Clean
└── homeserver / trafficsim / videos                             ──> Clean / Minimal
```

A final multi-repo verification sweep with `harnez scan-docs /home/uwe/projects` confirmed 100% clean alignment across the entire workspace.

---

## 4. Product Management & Restraint: Resisting the Mid-Flight Pivot

Midway through manually approving 28 repositories, the desire to immediately pause and code an automated batch runner (`/harnez-sync`) was strong.

In modern agentic development, this is a classic trap: **abandoning the primary operational goal to build meta-tooling mid-flight.**

We practiced deliberate product management discipline:
* **Finish the Active Milestone First**: We completed the manual interactive sweep. This ensured that all 28 repositories reached a clean, known-good baseline, proving the edge cases (like `ubunatic.com`'s traffic docs and `spriteview`'s GTK4 generalization) in reality rather than in simulation.
* **File a Structured Ticket for Later**: Instead of hijacking the session, we codified all the requirements, edge cases, and safety bounds into a durable backlog ticket: **[Issue 068 (`/harnez-sync`)](file:///home/uwe/projects/harnez/issues/068-harnez-sync-autonomous-doc-reconciliation-command.md)**.
* **Refine the Specification with Fresh Learnings**: Because the ticket was filed immediately after feeling the friction, the design for `/harnez-sync` captured subtle operational realities that an abstract initial design would have overlooked.

---

## 5. Designing for the Autonomous Agent: The `/harnez-sync` Blueprint

When designing developer tooling in an agentic harness, the most important design question is:
> *"How will an AI agent consume, reason over, and execute this command?"*

From our pairing session, four key design principles were baked into Issue 068:

```
                  /harnez-sync (Autonomous Blueprint)
                                   │
                                   ▼
                    [Autonomous Subagent Dispatch]
                                   │
              1. Discovery (., .., guard if .. is ~)
                                   │
              2. Multi-Repo Scan & Inbox Check (`scan-docs`)
                                   │
              3. Autonomous Diff Classification & Triage
                 ├── Pure Upstream Drift  ──> Auto-apply `harnez init -d <repo>`
                 ├── Local Customizations ──> Protect with `<!-- harnez:stop -->`
                 └── Upstream Promotions  ──> Skeptical Filter / Advisor Gate
                                   │
              4. Verification (`scan-docs <root>`)
                                   │
              5. Consolidated Final Report to Operator
```

### 5.1 Safety Boundary Guards
An agent executing discovery across `.` and `..` might accidentally treat `$HOME` (`~`) as a project container if the current directory is directly under home (e.g. `~/myproject`). Issue 068 explicitly specifies a safety boundary guard: if `..` resolves to `$HOME`, the scanner halts recursive ascent to prevent scanning the operator's entire home tree.

### 5.2 Conservative Upstream Promotion & The "False Generalization" Trap
A major insight from our session is that agents are prone to **over-generalization**. When an agent discovers a useful convention in one Go project (e.g. HTTP middleware logging patterns), it is tempted to promote it into `harnez/docs/lang/Go.md`.

However, Go projects vary wildly: a CLI utility like `fontwidth` has completely different constraints than a web service like `webman` or an OS process manager like `proctop`.

We codified strict guardrails in Issue 068:
1. **High Skepticism**: Treat candidate upstream promotions with skepticism.
2. **Capable Advisor Gate**: Require consultation with a top-tier capable advisor model (`pro`) before modifying canonical docs in `harnez/docs/`.
3. **Strict Default to Local Retention**: If the executing agent cannot spawn a subagent advisor, it **must strictly default to local retention** (preserving changes below `<!-- harnez:stop -->` in the local project) rather than promoting upstream.

### 5.3 CLI Ergonomics for Agents
Agents struggle with interactive terminal prompts (like `y/N` Makefile reconciliations). Issue 068 mandates that `harnez init` and batch reconciliation commands support non-interactive deterministic execution (`-y` / `--sync-present`) so subagents never stall in background execution.

---

## 6. Meta-Retrospective: How the Harness Performed

| Dimension | Observation | Assessment |
|---|---|---|
| **Harness Token Discipline** | Used `StartLine`/`EndLine` ranges and `grep_search` rather than whole-file reads on `AGENTS.md` or large docs. | Context remained responsive and compact throughout the long multi-repo sweep. |
| **Interactive Modals (`ask_question`)** | Used `ask_question` with structured options for each repository approval. | Provided clean, unambiguous operator checkpoints while preventing hallucinated inputs. |
| **Commit Hygiene** | Filed issue tickets (`issues/068-*.md` and `issues/README.md`) in dedicated, standalone metadata commits. | Kept git history clean, bisectable, and aligned with project standards. |
| **Drift Detection Accuracy** | `harnez scan-docs` accurately ignored trailing whitespace differences and post-stop-marker lines. | Eliminated diff noise, allowing the operator to focus entirely on genuine semantic differences. |

---

## 7. Conclusion

Building multi-repository agentic infrastructure requires a balance of strong centralized standards and resilient local domain protection. By grounding our mechanisms interactively across 28 diverse codebases, resisting the temptation to pivot mid-flight, and carefully specifying the autonomous future workflow in Issue 068, we established a clean foundation for fully autonomous multi-repo doc synchronization.
