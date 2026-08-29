# Autonomous Multi-Repo Managed-Docs Sync

Sweep sibling agent projects for managed-docs drift and reconcile it autonomously, without
per-repo confirmation prompts.

Reference Tickets: `issues/068-harnez-sync-autonomous-doc-reconciliation-command.md`,
`issues/archive/059-capture-managed-docs-drift-to-inbox.md`,
`issues/archive/060-triage-sibling-managed-docs-drift.md`,
`issues/archive/062-scan-docs-across-agent-projects.md`

---

## Invocation Syntax

- `/harnez-sync` — sweep `.` and `..` from the current directory.
- `/harnez-sync <path>` — sweep `<path>` and its immediate eligible children instead.

---

## Autonomous Workflow

Dispatch a fresh dev subagent to run the full sweep below to completion. Do not stop for
interactive per-repo prompts; every step must be resolvable from the rules in this file. Keep the
host orchestrator responsive — report the dispatch and do not block the main chat on the
subagent's result unless the user explicitly asked to wait.

### 1. Discovery
- Resolve the sweep root: the given `<path>`, or `.` when omitted.
- Also consider `..` (the parent workspace directory) as a second scan root, so sibling repos are
  reachable from inside any one of them.
- **Safety guard**: if `..` resolves to `$HOME`, do not scan `$HOME` and do not treat it as a
  project container. `harnez init --all` enforces this same guard at the CLI layer and will refuse
  outright if pointed at `$HOME` — rely on that refusal as a backstop, but do not attempt the scan
  in the first place.
- Eligibility for a child directory: it has `AGENTS.md` or `CLAUDE.md` (symlink or regular file).
  A `.git`-only directory is not eligible. This matches `harnez scan-docs`'s own eligibility rule.

### 2. Multi-Repo Scan & Inbox Check
- Run `harnez scan-docs <root>` for each sweep root from step 1.
- Read the combined report: clean repos, and repos with `changed` / `missing` / `extra` managed
  docs.
- Check `issues/inbox/` in the harnez repo (if reachable) for prior `managed-docs-drift-*.md`
  captures that overlap with the sweep root; reuse their findings instead of re-deriving them.

### 3. Autonomous Diff Classification & Triage
For each repo reporting drift, classify every changed/extra file into exactly one bucket:

- **Pure Upstream Drift** — the repo-local copy has fallen behind the harnez source doc with no
  repo-specific content mixed in (content above any `<!-- harnez:stop -->` marker only differs
  because the source moved on). Auto-apply the fix: `harnez init --docs <name> -d <repo>` for a
  single doc, or `harnez init --all <sweep-root>` when most/all eligible children in that root need
  the same reconciliation. Batch mode is intentionally non-interactive (no `-y` prompt loop).
- **Local Customizations** — the repo carries genuine repo-specific additions inside a
  harnez-owned doc. Before syncing, make sure those additions live below a `<!-- harnez:stop -->`
  marker (insert the marker above the local content if it is missing) so `harnez init` can update
  the managed portion without discarding local text. Then run the same `harnez init` step as above.
- **Upstream Promotions** — the local change looks like it should become the new upstream default
  (i.e. it is not really repo-specific). See the guardrails in step 3a before touching any file
  under `harnez/docs/`.

Zero-operator-prompting rule: every repo in a batch must be resolved to one of the three buckets
and acted on without stopping mid-sweep to ask the operator. Ambiguous cases default to Local
Customizations (see 3a) rather than blocking.

### 3a. Conservative Upstream Promotion Guardrails
- Treat promotion candidates with high skepticism. A pattern that looks good in one repo (or a
  pair of siblings) is often paradigm-specific (CLI vs. daemon vs. GUI, kernel vs. userland, etc.)
  rather than universal.
- Unless a promotion is overwhelmingly obvious and clearly universal, consult a highly capable
  advisor subagent (model `pro`, or the strongest model this harness can spawn) before editing any
  file under `harnez/docs/`. Give the advisor the specific diff and ask it to judge universality,
  not just correctness.
- If the executing subagent cannot spawn an advisor subagent for any reason, **default to Local
  Retention**: protect the change with `<!-- harnez:stop -->` (or move it into the repo's own
  evergreen docs) instead of promoting it upstream. Never promote on a coin flip.

### 4. Verification
- Re-run `harnez scan-docs <root>` for every sweep root touched in step 3.
- Confirm the repos that were auto-applied now report `identical` for the docs that were fixed.
- Confirm repos where local customizations were protected still show their local content present
  (i.e. nothing was silently deleted) and no longer report pre-marker drift.
- Run `go test ./...` in this repo if any `harnez` CLI code was touched during the sweep (it
  should not normally be — this command only *drives* the existing CLI).

### 5. Consolidated Final Report
Report back to the operator in one summary, not one message per repo:

- Sweep roots scanned, repo counts (eligible / skipped).
- Per-bucket counts: pure-upstream-drift auto-applied, local-customizations protected+synced,
  upstream-promotions (consolidated, or deferred to advisor/local-retention).
- Any repo left unresolved and why (e.g. advisor subagent unavailable and the change looked
  borderline enough that local retention alone felt unsatisfying — flag these for a human look
  rather than guessing).
- The verification result from step 4.

---

## Required CLI Support

This command relies entirely on existing `harnez` subcommands — it introduces no new drift-fixing
logic of its own:

- `harnez scan-docs <dir>` — read-only discovery/report (issue 062).
- `harnez init -d <repo> --docs <name>` — apply a single doc's upstream content to one repo.
- `harnez init --all <parent-dir>` — non-interactively reconcile every eligible child under
  `<parent-dir>` in one shot; refuses to run directly against `$HOME` (issue 068).
- The `<!-- harnez:stop -->` marker convention (issue 060) — everything above the marker in a
  harnez-owned doc is managed content; everything below is preserved local customization.

Do not reimplement scanning, diffing, or reconciliation logic inline in this workflow — always
shell out to the `harnez` CLI so the rules stay in one place.
