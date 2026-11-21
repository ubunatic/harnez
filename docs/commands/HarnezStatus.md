---
name: harnez-status
description: Briefly self-check Harnez CLI, model tiers, sprint workflow, and documentation navigation when explicitly invoked.
disable-model-invocation: true
---

# Harnez Status

When invoked, briefly report what you understand about these four areas. Do not run commands, inspect files, or change anything; this is a self-check based on your current context. If a detail is unknown, say so rather than guessing.

Output a compact checklist with one line per category and no conversational filler. Keep the full response under 15 lines.

- **CLI:** Recognize `harnez find`, `issues`, `read`, `usage`, `init`, `apply`, and `clean`; recognize `harnez agent start`, `resume`, `status`, `list`, `stop`, and `wait`.
- **Models and roles:** Know that `harnez agent models --picks` is the source for role picks; identify `--role dev --model lean` for standard development and `--role reviewer --model smart` for review.
- **Sprint workflow:** Summarize `/lean-sprint` as a zero-coding host workflow with single-ticket pre-work batching, diff-first review, and a plan-first gate.
- **Navigation:** Find evergreen docs in `docs/*.md`, the roadmap at `docs/Roadmap.md`, issues in `issues/*.md`, and shared practices in `docs/practices/`.

<!-- harnez:model-picks:begin -->
These are the shipped picks. User models.yaml overrides are listed by `harnez agent models --picks`.

| Role | Trait | Meaning | Chain | Default |
|---|---|---|---|---|
| advisor | cheap | Low-cost work. | luna:med, haiku:low, hydrogen:low | yes |
| advisor | lean | Efficient standard work. | terra:low, luna:high, neon:low, haiku:high |  |
| advisor | smart | Difficult judgment. | opus:high, neon:med, argon:high, sol:med |  |
| developer | cheap | Low-cost work. | luna:med, haiku:low, hydrogen:low |  |
| developer | fast | Fast work. | haiku:low, luna:med, hydrogen:low |  |
| developer | gui | Graphical interfaces. | lean, std |  |
| developer | lean | Efficient standard work. | terra:low, luna:high, haiku:high, neon:low | yes |
| developer | smart | Difficult judgment. | sol:med, opus:high, neon:med |  |
| developer | std | Standard work. | sol:low, luna:high, haiku:xhigh, neon:low |  |
| developer | tui | Terminal interfaces. | lean |  |
| reviewer | best | Hardest judgment, highest cost. | sol:high, neon:high, argon:high, fable:med |  |
| reviewer | cheap | Low-cost work. | haiku:low, luna:med, hydrogen:low |  |
| reviewer | lean | Efficient standard work. | terra:low, luna:high, neon:low, haiku:high |  |
| reviewer | smart | Difficult judgment. | sol:med, neon:med, opus:high | yes |
<!-- harnez:model-picks:end -->
