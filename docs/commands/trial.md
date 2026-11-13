# Trial: 10-5 Capability Audit

Run an objective, evidence-backed capability trial over recent work, extracting **ten concrete things the product can do now** and **five clear things it cannot do yet**.

Triggers: `/trial`, `/trial <scope>` (e.g. `/trial today`, `/trial sprint`, `/trial <ticket>`), or when the user asks for a capability assessment or progress audit.

---

## 1. Invocation Syntax

- `/trial` or `/trial today` — evidence: `git log --since="midnight" --oneline` (use `--since="24 hours ago"` if empty), plus tickets closed in that window via `harnez find -d . issues closed` (returns the highest-numbered 10; never use `-a status:closed`, which dumps every closed ticket). Verify each ticket's close commit date against the window with `git log --since="midnight" -- issues/`.
- `/trial sprint` — evidence: `git log main..HEAD --oneline` on the current topic/sprint branch, plus tickets closed on this branch via `git log main..HEAD -- issues/`.
- `/trial <ticket>` — evidence: `git log --grep="<ticket>" --oneline` and `harnez issues show -d . <ticket>`.

---

## 2. Evidence Gathering Workflow

A `/trial` audit must be grounded in primary project evidence, not subjective opinion or hypothetical claims:

1. **Inspect Delivered Capabilities**:
   - Check commit history with the scoped `git log` command for the chosen scope from Section 1.
   - Check closed tickets with the scoped commands from Section 1 (`harnez find -d . issues closed`, `git log ... -- issues/`), and keep only tickets whose close commit falls inside the target window.
   - Identify concrete features, new CLI flags, verified bug fixes, performance optimizations, and hardened guards.
2. **Inspect Active Boundaries & Limitations**:
   - Check open issues and active blockers via `harnez find -d . issues -a status:active`, keeping only those that relate to the scoped features (active gaps, missing CLI verbs, unresolved follow-ups of the scoped tickets), not unrelated backlog items.
   - Identify unverified assumptions, pending canary tests, open defects, or missing CLI verbs.
   - Note external boundaries (unsupported platforms, pending upstream forge features, or API quotas).

---

## 3. Output Format

- format the audit concise and structured with zero opening/closing prose
- use exact markdown links to code symbols, commits, and issue tickets
- if applicable say what the **user** can do with the new product features or cannot do yet
- if applicable say what the outcome of using the new features is
- add a final conclusion how much the changes brought the product forward

---

## 4. Execution Rules

- **Verified "Can Do"**: Every item must cite a CLI command, flag, or named test and you must confirm it exists at HEAD (e.g. `harnez <cmd> --help`, `grep` for the flag, or `go test -run <Name>`). Do not just repeat commit subjects. Exclude pure internal refactors, and list at most one item per ticket.
- **Direct test evidence**: The cited test must directly exercise the claimed capability, not a neighbouring one.
- **No overclaiming**: If a cited ticket is still Open or records outstanding gaps, the "Can Do" item must state that status and the limited scope (e.g. "Open: auto-compaction gaps remain") instead of claiming full completion.
- **Cited "Cannot Do"**: Every item must relate to the scoped features and cite an open ticket (from `harnez find -d . issues -a status:active`, or a Backlog ticket) or a command failure you reproduced in this session (quote the command and error).
- **Shortfall rule (both lists)**: If fewer than 10 genuine capabilities or fewer than 5 genuine limitations exist in the requested scope, state the verified count honestly (e.g. "6 verified") rather than padding with trivial, duplicate, or unverified items.
- **Concise Discipline**: Do not add conversational filler. Present verbatim CLI commands and direct evidence.
