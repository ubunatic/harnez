---
title: Feature Prototyping
weight: 25
---

<!-- harnez:bundled -->
# Feature Prototyping

Harnez's canary-first practice is its feature-prototyping and mechanism-isolation
practice. When a feature depends on an unknown external tool, protocol, terminal
behaviour, or operating-system facility, make the smallest standalone canary,
run it against the real mechanism, record what it proves, and keep it as a
re-runnable artifact before building the feature. See `@docs/Canary.md`.

This is the project's version of a kept tracer bullet or technical spike. It
avoids a second, competing "prototype" workflow while retaining useful output
for later agents. Other harnesses use related runtime mechanisms: Codex uses
worktrees and sandboxes, Devin uses sandboxed or per-agent environments, and
tracer-bullet practices sometimes compare several implementations side by side.

Canary-first does not claim to replace:

- runtime isolation such as worktrees, sandboxes, or VMs;
- feature flags for staged rollout of an already-built feature; or
- parallel comparison of competing implementations.

Those are separate design or delivery concerns. Use them when needed, but keep
the canary as the evidence for whether the underlying mechanism actually works.
