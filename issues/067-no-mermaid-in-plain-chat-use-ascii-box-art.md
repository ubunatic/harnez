# 067 — Do Not Render Mermaid in Plain Chat; Use ASCII Box/Arrow Art (Mermaid Reserved for Docs Only)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics & UI Standards
**Related**: [[039-agentic-loop-practices-and-sprint-command]], `docs/lang/Markdown.md`, `docs/practices/AgenticLoop.md`

---

## 1. Problem & Motivation

In chat and interactive pairing sessions, agents frequently output raw ````mermaid``` code blocks. In many terminal interfaces, chat streams, or simple markdown renderers, Mermaid blocks render as raw code or unrendered markup, cluttering the stream and reducing immediate legibility.

Mermaid diagrams are valuable in persistent evergreen documentation (`docs/`), but during active conversation and chat responses, lightweight ASCII box-and-arrow diagrams (`[ Box ] --> [ Box ]` or `┌──┐ └──┘`) are immediately readable, copy-pasteable, and render deterministically across all terminal clients and chat UIs.

---

## 2. Technical Specification & Conventions

### 2.1 Communication Convention Update (`docs/lang/Markdown.md`)

- **Chat vs. Doc Diagram Rules**:
  - **In Chat / Pairing Conversations**: **Never emit ````mermaid``` blocks**. Use readable, compact ASCII box-and-arrow diagrams inside standard ````text``` code fences.
  - **In Evergreen Documentation (`docs/`)**: Mermaid diagrams (````mermaid```) remain allowed and encouraged for formal system architecture and sequence flows.

### 2.2 Standard ASCII Box/Arrow Format

```text
┌─────────────────────────┐        ┌─────────────────────────┐
│     Upstream Input      │ ─────> │     Process / Filter    │
└─────────────────────────┘        └─────────────────────────┘
                                                │
                                                ▼
                                   ┌─────────────────────────┐
                                   │      Final Output       │
                                   └─────────────────────────┘
```

---

## 3. Implementation & Verification Plan

1. Update `docs/lang/Markdown.md` with explicit chat vs. doc formatting rules.
2. Update `AGENTS.md` conventions in `harnez` templates.
3. Validate via `harnez status` and mark closed.
