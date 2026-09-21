# 156 — Document how `collaboration.spawn_agent` appears in Codex Agents

**Status**: Closed — Documented Codex spawn_agent Agents view delegation and lifecycle responsibilities
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: [[144-codex-subagent-model-selection-policy]], [[145-orchestrator-session-skill-and-command]], [[149-agent-specific-profiles-codex-async-wait-instruction]], `docs/AgenticLoop.md`, Codex collaboration tools

---

## 1. Problem & Motivation

Codex users can see delegated work in the Codex Agents view, but the current
project documentation does not explicitly connect that UI effect to the
`collaboration.spawn_agent` tool. The relationship was directly observed when
ticket 155 was handed to the `implement_155` agent: dispatching the tool made
that agent appear in the view.

Document the observable behavior so hosts can deliberately use delegation when
a visible, independently tracked work stream is useful, and can accurately
explain the result to users.

## 2. Technical Specification / Findings

- `collaboration.spawn_agent` starts a named child agent; it is the action
  that creates the corresponding entry in Codex Agents for the current agent
  tree.
- The visible entry provides a user-facing status surface for delegated work;
  it does not replace host responsibility for status updates, review,
  integration, or lifecycle hygiene.
- Document the relationship in the Codex-specific operational guidance (or a
  clearly linked shared orchestration document), including agent naming and
  the distinction between spawning, messaging/follow-up, and termination.
- Keep the guidance product-accurate and avoid promising UI details that have
  not been verified by the current Codex environment.

## 3. Implementation & Verification Plan

- [ ] Add concise Codex documentation identifying `collaboration.spawn_agent`
  as the mechanism that creates an entry in Codex Agents.
- [ ] Explain the practical effect: users can observe the named child and its
  progress there while the host remains responsive.
- [ ] Cross-link existing subagent lifecycle/model-selection guidance without
  duplicating it.
- [ ] Review the resulting documentation for accurate tool names and bounded
  claims about the UI.

---

## 4. Implementation Plan

### Research notes (2026-09-04)

- There is **no Codex-specific doc in this repo today**. Codex appears only in
  `docs/CommandsPipeline.md` (skill-target plumbing: `~/.codex/skills/<name>/SKILL.md`) and in two
  dated `docs/studies/` reports. So this ticket has no existing home to append to and must pick one.
- `docs/practices/AgenticLoop.md` is the shared orchestration doc the ticket alludes to. Its §5
  Role Taxonomy table and §6 Anti-Patterns (which already carries **Blocking Handoff Waits** —
  "the host is always the responsive orchestrator") are the natural anchor: the Codex Agents view
  is precisely the surface that makes a non-blocking handoff *visible* to the user.
- `AgenticLoop.md` is a **copyable/bundled doc** (`docs/practices/`, installed to `~/.claude/docs/`
  by `apply`, per `config.yaml:469`). Anything added there reaches every agent on every project —
  so only *descriptive, harness-comparative* content belongs there, not a Codex-only imperative.
- [[149]] is building `agents_md.agents.<id>` for exactly the Codex-only imperative case. This
  ticket should not wait on it (it is documentation, not instruction), but should be written so the
  imperative half can migrate cleanly once 149 lands.

### Steps

1. **Verify before writing** (the ticket's own bounded-claims requirement). In a live Codex
   session: dispatch `collaboration.spawn_agent` with an explicit name, then record what actually
   appears in the Agents view — entry name, status states shown, whether progress updates live,
   whether the entry persists after the child finishes, and what messaging/termination tools exist
   alongside spawn. Write down only what was observed; mark anything unobserved as unverified.
   Do not write step 2 from memory of the ticket 155 handoff.
2. **Add a new subsection to `docs/practices/AgenticLoop.md`**, placed after §5 (Role Taxonomy),
   titled something like *"Harness-Specific Delegation Surfaces"*. Keep it to ~10-15 lines and
   make it comparative rather than Codex-only, since the doc ships to all agents:
   - Claude Code: subagent dispatch, host stays responsive, host is notified on completion.
   - Codex: `collaboration.spawn_agent` starts a named child **and** creates the corresponding
     entry in the Codex Agents view — this is the user-visible status surface for delegated work.
   - The shared rule (applies regardless of harness): a visible child entry is a *status surface*,
     not a delegation of responsibility — the host still owns status reporting, review,
     integration, and termination (cross-link §6's Zero Zombie Guarantee and Blocking Handoff
     Waits rather than restating them).
   - Naming guidance: name children after the work (`implement_155`), not after the model or role,
     so the Agents view is scannable.
3. **Distinguish the three verbs explicitly** in that subsection — spawn (creates the entry),
   message/follow-up (updates an existing child), terminate (ends it) — since the ticket calls this
   out and it is the part most likely to be conflated.
4. **Cross-link, do not duplicate**: reference [[144]] for model selection and §5/§6 for lifecycle
   hygiene. No new copy of either.
5. **Regenerate/verify doc plumbing**: `docs/README.md` is generated (`harnez index`) — if the new
   subsection changes the doc's extracted topics, the index needs a refresh in the same commit.
   Run `harnez apply` afterwards so the bundled copy in `~/.claude/docs/AgenticLoop.md` matches.
6. **Follow-up hook for [[149]]**: if 149 lands first (or soon after), move any *imperative* Codex
   phrasing ("prefer spawn_agent when a visible work stream helps") into `agents_md.agents.codex`
   and leave only the descriptive comparison in `AgenticLoop.md`. Note this in the subsection as a
   one-line comment for the next editor.

### Design decisions / tradeoffs

- **Shared comparative doc over a new Codex-only doc.** A standalone `docs/other/Codex.md` would be
  a third copyable doc category entry for ~15 lines of content, and the repo's own layout rule says
  a category forms at 3+ docs. The comparison also has genuine value for Claude Code readers
  deciding whether a delegation is visible to the user.
- **Comparative framing keeps the bundled doc honest**: a Codex-only paragraph shipped to Claude
  Code is exactly the noise [[149]] exists to eliminate.

### Risks / open questions

- **Product drift**: Codex UI details can change; keep claims to the causal relationship
  (spawn → entry appears) and avoid describing UI chrome, columns, or state names that could go
  stale.
- Step 1's verification may show the relationship is more conditional than ticket 155's single
  observation suggested (e.g. only certain spawn modes surface). If so, weaken the claim rather
  than dropping the ticket.

### Scope estimate

**Small** — one verification session plus a ~15-line doc subsection and an index refresh.
