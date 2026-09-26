---
name: harnez-status
description: Briefly self-check Harnez CLI, model tiers, sprint workflow, and documentation navigation when explicitly invoked.
disable-model-invocation: true
---

# Harnez Status

When invoked, briefly report what you understand about these four areas. Do not run commands, inspect files, or change anything; this is a self-check based on your current context. If a detail is unknown, say so rather than guessing.

Output a compact checklist with one line per category and no conversational filler. Keep the full response under 15 lines.

- **CLI:** Recognize `harnez find`, `issues`, `read`, `usage`, `init`, `apply`, and `clean`; recognize `harnez agent start`, `resume`, `status`, `list`, `stop`, and `wait`.
- **Models and roles:** Know that `harnez agent models` is the source for aliases and cost tiers; identify `luna:med` as the preferred developer/worker tier and `terra:med` as the preferred reviewer/auditor tier.
- **Sprint workflow:** Summarize `/lean-sprint` as a zero-coding host workflow with single-ticket pre-work batching, diff-first review, and a plan-first gate.
- **Navigation:** Find evergreen docs in `docs/*.md`, the roadmap at `docs/Roadmap.md`, issues in `issues/*.md`, and shared practices in `docs/practices/`.
