# 077 — Box Diagram Width Limits & Right-Side Padding Rules (Prevent Terminal Line Wrap Collapse)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics & UI Standards
**Related**: [[067-no-mermaid-in-plain-chat-use-ascii-box-art]], `docs/lang/Markdown.md`, `docs/practices/AgenticLoop.md`

---

## 1. Problem & Motivation

ASCII box-and-arrow diagrams (`┌───┐`, `│...│`, `└───┘`) render cleanly in chat and documentation, but when lines approach or exceed the terminal or chat pane width (typically 75–80+ characters), terminal line-wrapping breaks the rectangular borders.

When a single border line wraps onto the next line:
1. Box borders stagger and collapse vertically into jagged, unreadable lines.
2. Padding alignment is ruined across all subsequent boxes.
3. Terminal emulators and chat sidebars with margins, scrollbars, or imperfect character column calculations suffer severe rendering artifacts.

---

## 2. Technical Specification & Markdown Convention

### 2.1 Max Width & Right Margin Invariant
- **Max Width**: ASCII box diagrams in chat MUST not exceed **65–70 characters** in total line length.
- **Right Margin Safety**: Always leave a buffer of at least **10–15 columns** on the right so terminals, split panes, and IDE chat sidebars have room without triggering auto-wrap.
- **Stacking over Widening**: If multiple boxes or columns are needed, stack components vertically instead of chaining them horizontally across the full terminal width.

### 2.2 Formatting Example

**Avoid (Too wide, spans > 80 cols, will collapse on wrap):**
```text
┌───────────────────────────────────────┬───────────────────────────────────────┐
│              Long Box A               │              Long Box B               │
└───────────────────────────────────────┴───────────────────────────────────────┘
```

**Prefer (Compact width <= 65 cols, safe right margin):**
```text
┌─────────────────────────┐
│         Step 1          │
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│         Step 2          │
└─────────────────────────┘
```

---

## 3. Implementation Plan

1. Update `docs/lang/Markdown.md` with explicit box width ($\le 65\text{--}70$ chars) and right-margin safety rules.
2. Update agent system prompts and markdown formatting guidelines.
