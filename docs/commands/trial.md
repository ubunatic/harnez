# Trial: 10-5 Capability Audit

Run an objective, evidence-backed capability trial over recent work, extracting **ten concrete things the product can do now** and **five clear things it cannot do yet**.

Triggers: `/trial`, `/trial <scope>` (e.g. `/trial today`, `/trial sprint`, `/trial <ticket>`), or when the user asks for a capability assessment or progress audit.

---

## 1. Invocation Syntax

- `/trial` or `/trial today` — evidence: `git log --since="midnight" --oneline` (use `--since="24 hours ago"` if empty), plus tickets closed in that window via `harnez find -d . issues -a status:closed` (newest 10 only; never dump the whole closed index).
- `/trial sprint` — evidence: `git log main..HEAD --oneline` on the current topic/sprint branch, plus tickets closed on this branch.
- `/trial <ticket>` — evidence: `git log --grep="<ticket>" --oneline` and `harnez issues show -d . <ticket>`.

---

## 2. Evidence Gathering Workflow

A `/trial` audit must be grounded in primary project evidence, not subjective opinion or hypothetical claims:

1. **Inspect Delivered Capabilities**:
   - Check recent git commit history (`git log --since="24 hours ago" --oneline` or `git log -n 25 --oneline`).
   - Check recently closed tickets in `issues/CLOSED.md` or via `harnez find issues -a status:closed`.
   - Identify concrete features, new CLI flags, verified bug fixes, performance optimizations, and hardened guards.
2. **Inspect Active Boundaries & Limitations**:
   - Check open issues and active blockers via `harnez find issues -a status:active`.
   - Identify unverified assumptions, pending canary tests, open defects, or missing CLI verbs.
   - Note external boundaries (unsupported platforms, pending upstream forge features, or API quotas).

---

## 3. Output Format

Format the audit with sharp contrast, zero opening/closing pleasantries, and exact markdown links to code symbols, commits, and issue tickets:

```markdown
# ⚖️ Capability Trial: <Scope / Product Name>

### Ten Things We Can Do Now

1. **<Feature / Capability Name>** — <Brief description of what works, citing exact CLI command or flag and linking commit/ticket>.
2. **<Feature / Capability Name>** — <Brief description, linking commit/ticket>.
...
10. **<Feature / Capability Name>** — <Brief description, linking commit/ticket>.

---

### Five Things We Cannot Do Yet

1. **<Limitation / Gap Name>** — <Concrete description of what fails, what is blocked, or what command does not exist, linking open ticket>.
2. **<Limitation / Gap Name>** — <Concrete description, linking open ticket>.
...
5. **<Limitation / Gap Name>** — <Concrete description, linking open ticket>.
```

---

## 4. Execution Rules

- **Verified "Can Do"**: Every item must cite a CLI command, flag, or named test and you must confirm it exists at HEAD (e.g. `harnez <cmd> --help`, `grep` for the flag, or `go test -run <Name>`). Do not just repeat commit subjects. Exclude pure internal refactors, and list at most one item per ticket.
- **Cited "Cannot Do"**: Every item must cite an open ticket (from `harnez find -d . issues -a status:active`, or a Backlog ticket) or a command failure you reproduced in this session (quote the command and error).
- **Shortfall rule (both lists)**: If fewer than 10 genuine capabilities or fewer than 5 genuine limitations exist in the requested scope, state the verified count honestly (e.g. "6 verified") rather than padding with trivial, duplicate, or unverified items.
- **Concise Discipline**: Do not add conversational filler. Present verbatim CLI commands and direct evidence.
