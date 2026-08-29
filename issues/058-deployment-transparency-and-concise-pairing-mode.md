# 058 — Deployment Transparency, Live State Grounding, and Concise Pairing Mode

**Status**: Closed — resolved 2026-08-29
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Practices / Conventions / Tooling
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md), [057-repo-assessment-and-code-metrics-command.md](057-repo-assessment-and-code-metrics-command.md)

---

## 1. Problem & Motivation

During remote infrastructure provisioning and live deployment pairing in sibling projects (e.g. `webman`), two major friction points emerged:

1. **Operator Over-Verbosity**:
   - Agents default to verbose, essay-style narrative prose during operational dialogues.
   - Long explanations create cognitive fatigue for human operators and bury crucial factual gaps (such as uninstalled binaries or missing crontab entries) under paragraphs of reassuring text.
   - When requested to adopt high-density reporting ("3–7 one-line bullets"), operator alignment and cycle speed improved dramatically.

2. **Grounding Gaps between Local Code and Remote Runtime**:
   - Agents frequently conflate local codebase state, remote deployed filesystem state, and active daemon/scheduler state.
   - Example: After refactoring a CLI from `weg` to `webman`, local Go builds succeeded, but the remote host still had the legacy `~/bin/weg` binary on disk and an empty crontab. The agent explained how the remote runner worked based on local code assumptions rather than probing live remote reality via SSH.
   - Example: When public repository sanitization (Issue 024) replaced production YAML values with `example.com` fixtures, the remote daemon fell back to the dummy embedded spec because the remote backup resolution path was unverified.

---

## 2. Goals & Proposed Features

### 2.1 Concise Operational Pairing Mode (Skill / Command / Rule)
- Introduce an opt-in customization (e.g. `/concise` slash command, `concise-ops` skill, or interactive pairing rule) for AI agent harnesses.
- When active, responses default strictly to **3–7 one-line bullet points**:
  - Fact / Command run
  - Status / Exit code / Observation
  - Gaps / Discrepancies found
  - Next recommended action

### 2.2 Deployment Transparency Invariant in `AgenticLoop.md`
- Update `docs/practices/AgenticLoop.md` to establish a strict 3-state grounding rule for remote environments:
  1. **Local State**: Local git checkout, configs, unit tests.
  2. **Deployed Artifact State**: Remote filesystem binaries, permissions, `.env` files, config overlays.
  3. **Active Daemon State**: Remote process table, systemd units, active crontab entries.
- Invariant: Agents must never declare remote deployment status without probing the live host over SSH (`crontab -l`, `ls -la`, `file`, `head`).

### 2.3 Spec Decoupling Integration Test Pattern
- Invariant in `docs/Spec.md`: When repository data is decoupled into public fixtures (`spec.yaml`) and private production overlays (`spec.local.yaml` / `~/.<app>/backup/current/webspace.yaml`), an integration test must verify that the daemon / cron runner resolves the private overlay in isolation without falling back to public placeholder fixtures.

### 2.4 Make Target Parity
- Every project with mutating provisioners must provide standard, self-documenting Make targets:
  - `make deploy [DRY=1]` — Deploy binary, configs, and cron schedules.
  - `make run` / `make status` — Query live deployment health.
  - `make backup` — Sync state snapshots.

---

## 3. Acceptance Criteria

- [x] Add `docs/practices/DeploymentTransparency.md` convention to `harnez` bundle.
- [x] Update `docs/practices/AgenticLoop.md` with the 3-state grounding rule.
- [x] Add optional `concise` pairing mode specification or slash command definition to `harnez`.
- [x] Add deployment parity rules to `docs/lang/Make.md`.

---

## 4. Progress (2026-08-29)

Implemented as a pure docs change; no Go code touched (docs are auto-discovered via the existing
`//go:embed docs/lang docs/other docs/practices ...` directive in `embed.go`, so the new file
needed no registration).

- **`docs/practices/DeploymentTransparency.md`** (new): defines the 3-state grounding rule (Local
  State / Deployed Artifact State / Active Daemon State), the invariant that remote deployment
  status must be probed live over SSH rather than assumed, minimum probes per state, and
  anti-patterns — sourced directly from the `webman` `weg`→`webman` stale-binary and
  `example.com`-fallback incidents described in §1.
- **`docs/practices/AgenticLoop.md`**: added invariant #8 ("Deployment Transparency — 3-State
  Grounding") to §1 Core Philosophy & Invariants, cross-linking the new doc; added a matching
  "Deployment State Conflation" row to the §6 Anti-Patterns list.
- **`docs/practices/ConciseMode.md`**: extended (not duplicated/forked) with a new "Operational
  Pairing Format" section documenting the 3–7-bullet `Fact / Status / Gaps / Next action`
  structure for live deployment/pairing sessions, distinct from and composable with the existing
  three narrative terseness tiers. Reused the existing `/mode` command
  (`commands/mode.md`) as the invocation mechanism rather than adding a new slash command, since
  it already toggles `ConciseMode.md` tiers by reference.
- **`docs/lang/Make.md`**: added a "Deployment target parity" section specifying the standard
  `make deploy [DRY=1]` / `make run` / `make status` (alias) / `make backup` targets for any
  project with mutating provisioners, cross-linked to `DeploymentTransparency.md`.
- **`docs/README.md`**: added an index row for `practices/DeploymentTransparency.md` under the
  `docs/practices/` table so it's discoverable like the other copyable practice docs.

Out of scope (not in the ticket's Acceptance Criteria list): §2.3's `docs/Spec.md` integration-test
proposal was deliberately left untouched — it isn't one of the four checked acceptance criteria,
and 057/other concurrent tickets touch adjacent areas.
