---
title: Deployment Transparency Practices
weight: 47
---

# Deployment Transparency — Live State Grounding

Agents that provision or pair on remote infrastructure routinely conflate three distinct layers of
"truth": what the local checkout says, what actually landed on the remote host, and what is
actually running there right now. This doc establishes an invariant that closes that gap.

Origin: proposed from a `webman` pairing retrospective (see
an early deployment-transparency pairing incident) after two
concrete failures — a stale `~/bin/weg` binary surviving a `weg` → `webman` rename because the
remote host was never re-probed, and a daemon silently falling back to a public `example.com`
fixture after spec sanitization because the private overlay resolution path was never verified
live.

---

## The 3-State Grounding Rule

Remote deployment status has three independent states. An agent must not describe or assume one
from another — each is only known once it has actually been probed.

1. **Local State** — the local git checkout: working tree, configs, unit test results. This is
   what the agent's own edits directly control.
2. **Deployed Artifact State** — what is actually on the remote filesystem: binaries, permissions,
   `.env` files, config overlays. Local build success says nothing about this.
3. **Active Daemon State** — what is actually running or scheduled on the remote host: process
   table entries, systemd units, active crontab lines. A correctly deployed artifact says nothing
   about whether it is wired up to run.

**Invariant**: Agents must never declare remote deployment status (deployed, running, scheduled,
current) without probing the live host over SSH. Passing local tests or a clean local build is
evidence about Local State only, not about Deployed Artifact State or Active Daemon State.

Minimum probes per state, run over SSH against the actual host:

- Deployed Artifact State: `ls -la`, `file`, `head` / `sha256sum` on the deployed binary or config
  to confirm it matches what was intended to ship (not a stale prior version).
- Active Daemon State: `crontab -l`, `systemctl status` / `systemctl list-timers`, `ps aux | grep`
  to confirm the thing that's on disk is actually scheduled or running.

If a probe cannot be run (no SSH access, host unreachable), say so explicitly rather than
extrapolating from local state — e.g. "cannot verify remote crontab; last local push was at
<time>" rather than "the cron job is running."

## Anti-Patterns

- Explaining how a remote process behaves by reading the local source, without confirming the
  remote host is running that version.
- Treating "the deploy command exited 0" as proof of Active Daemon State — a successful `scp`/build
  step proves nothing about whether a service picked up the new artifact.
- Assuming a config decoupling (public fixture vs. private overlay) resolved correctly on the remote host just because it
  resolves correctly in local tests — verify the daemon actually loaded the private overlay, not
  the embedded public fallback.

## Relation to Other Practices

- The Agentic Loop practice's Canary & Test-Driven Verification invariant already
  requires real test execution before declaring completion; this doc extends that requirement
  explicitly to remote/deployed state, which local test runs cannot cover.
- The optional Concise Mode practice documents the Operational Pairing bullet format
  for reporting probe results tersely during live deployment sessions.
- The Make practice documents the `make deploy` / `make status` target
  convention used to make these probes repeatable rather than ad hoc SSH one-liners.
