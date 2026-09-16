# Initial RAMP Assessment: Sibling Repositories

**Date**: 2026-09-17  
**Evaluator**: Harnez RAMP Scanner (`ramp-v1`)  
**Scope**: Sibling repositories in workspace (`cati`, `psync`, `loom`, `uman`, `harnez.org`, `voxi`, `termaid`, and `harnez`)  
**Related**: [`docs/studies/2026-09-16-ramp-levels-and-go-repository-init.md`](studies/2026-09-16-ramp-levels-and-go-repository-init.md), [[368-report-ramp-evidence-and-projected-changes-from-harnez-init]], [[384-harnez-assess-directory-validation-and-d-dir-flag-support]]

---

## 1. Executive Summary

An initial RAMP (Repository AI Maturity Profile) assessment was executed across all active sibling projects using the newly shipped offline, read-only scanner (`harnez assess <repo> --ramp`).

All active repositories scored **L2 (Grounded Prompting)** with zero coherence warnings. Every project maintains standard canonical agent instructions (`AGENTS.md`, `CLAUDE.md`) and normative practices (`docs/AgenticLoop.md`, `docs/IssueTracking.md`). None currently commit project-specific L3 capabilities (reusable commands, domain skills) or L4 orchestration artifacts (multi-agent pipelines, session logs).

| Repository | Path | Baseline Level | Projected Level | Coherence | Primary Evidenced Artifacts |
| :--- | :--- | :---: | :---: | :---: | :--- |
| **harnez** | `.` | **L2** | **L2** | ✓ Coherent | `AGENTS.md`, `CLAUDE.md`, `docs/AgenticLoop.md`, `docs/IssueTracking.md` |
| **cati** | `../cati` | **L2** | **L2** | ✓ Coherent | `AGENTS.md`, `CLAUDE.md`, `docs/AgenticLoop.md`, `docs/IssueTracking.md` |
| **psync** | `../psync` | **L2** | **L2** | ✓ Coherent | `AGENTS.md`, `CLAUDE.md`, `docs/AgenticLoop.md`, `docs/IssueTracking.md` |
| **loom** | `../loom` | **L2** | **L2** | ✓ Coherent | `AGENTS.md`, `CLAUDE.md`, `docs/AgenticLoop.md`, `docs/IssueTracking.md` |
| **uman** | `../uman` | **L2** | **L2** | ✓ Coherent | `AGENTS.md`, `CLAUDE.md`, `docs/AgenticLoop.md`, `docs/IssueTracking.md` |
| **harnez.org** | `../harnez.org` | **L2** | **L2** | ✓ Coherent | `AGENTS.md`, `CLAUDE.md`, `docs/AgenticLoop.md`, `docs/IssueTracking.md` |
| **voxi** | `../voxi` | **L2** | **L2** | ✓ Coherent | `AGENTS.md`, `CLAUDE.md`, `docs/AgenticLoop.md`, `docs/IssueTracking.md` |
| **termaid** | `../termaid` | **L2** | **L2** | ✓ Coherent | `AGENTS.md`, `CLAUDE.md`, `docs/AgenticLoop.md`, `docs/IssueTracking.md` |

---

## 2. Detailed Repository Observations

### 2.1 Uniform L2 Baseline & Practice Distribution
- All repositories have undergone standard `harnez init` and sync, establishing high-quality **L2 Grounded Prompting**.
- Standard practices (`AgenticLoop.md`, `IssueTracking.md`) are consistently tracked and committed.
- No repositories exhibited broken or ungrounded higher-level artifacts (e.g., orphan skills without rules).

### 2.2 Precision & False-Positive Rejection
The offline rule set demonstrated robust discrimination against common false positives:
- **`voxi`**: Contains internal Go application packages (`internal/agent/agent.go`) and systemd units (`systemd/voxi-agent.service`). The detector correctly identified these as application logic and service infrastructure rather than AI agent artifacts, preventing false L3/L4 promotion.
- **`harnez.org`**: Contains static documentation HTML under `skills/*/index.html`. These were correctly distinguished from runnable/structured skill definitions (`SKILL.md`).
- **`harnez`**: Contains `.agents/hooks.json` and `scripts/agent-canary/run.sh`. Because these are auxiliary hook/canary harnesses rather than standard agent declarations, they were categorized with `unknown` confidence under L1 without score inflation.

---

## 3. Discovered Issues & Follow-Ups

1. **Missing Directory Validation in `harnez assess`**
   - *Observation*: Querying a non-existent path (e.g. `harnez assess ../psyc` due to typo) did not error out; it silently treated the path as a repository lacking Git metadata and returned L1 Unconfigured.
   - *Follow-up*: Filed issue **[[384-harnez-assess-directory-validation-and-d-dir-flag-support]]**.

2. **CLI Flag Consistency (`-d, --dir`)**
   - *Observation*: While commands such as `harnez init`, `harnez issues`, and `harnez find` support `-d, --dir <path>`, `harnez assess` only supported positional arguments.
   - *Follow-up*: Bundled into issue **[[384-harnez-assess-directory-validation-and-d-dir-flag-support]]**.

3. **Pathway to L3 Capabilities for Go Repositories**
   - *Observation*: To genuinely progress from L2 to L3, repositories require committed, task-specific agent commands and domain skills rather than empty persona files or decorative workflows.
   - *Follow-up*: Tracked under existing issue **[[370-offer-useful-go-agent-capabilities-through-harnez-init]]**.
