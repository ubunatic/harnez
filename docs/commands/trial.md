# Trial: 10-5 Capability Audit

Run an objective, evidence-backed capability trial over recent work, extracting **ten concrete things the product can do now** and **five clear things it cannot do yet**.

Triggers: `/trial`, `/trial <scope>` (e.g. `/trial today`, `/trial sprint`, `/trial <ticket>`), or when the user asks for a capability assessment or progress audit.

---

## 1. Invocation Syntax

- `/trial` or `/trial today` — audit capabilities and boundaries delivered today (or over the active session).
- `/trial sprint` or `/trial <ticket>` — audit capabilities and boundaries delivered for a specific sprint, milestone, or epic.

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

- **Zero Hallucination**: Every "Can Do" item must correspond to existing, committed, and test-verified code. If fewer than 10 genuine capabilities exist in the requested scope, state the exact verified count honestly rather than padding with trivial or duplicate items.
- **Actionable Limitations**: Every "Cannot Do" item must represent a real functional gap or known defect, preferably linking to an open issue in `issues/` or proposing a follow-up ticket.
- **Concise Discipline**: Do not add conversational filler. Present verbatim CLI commands and direct evidence.
