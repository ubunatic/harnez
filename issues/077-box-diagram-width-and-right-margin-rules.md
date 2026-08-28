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

### 2.1 Dynamic Width & 5% Safety Buffer Invariant
- **Dynamic Terminal Sizing**: Size box diagrams to fit the current terminal or chat pane width $W$.
- **5% Right Margin Safety Buffer**: Always leave at least a **5% buffer** on the right side (e.g., width $\le 0.95 \times W$) so terminals with slight margin differences, scrollbars, or split panes never trigger line-wrapping.
- **Hard Maximum Ceiling of 120 Columns**: Even on ultra-wide displays ($W > 120$), cap the diagram width at **120 columns** for readability and clean terminal rendering.
- **Narrow Terminal Adaptation**: On standard 80-column terminals, keep diagrams to $\le 76$ columns ($80 - 5\%$).

### 2.2 Formatting Rule
- If the diagram would naturally exceed the safe width ($0.95 \times W$ or $120$ cols), stack components vertically rather than spreading wide horizontally.

---

## 3. Implementation Plan

1. Update `docs/lang/Markdown.md` with explicit box width ($\le 65\text{--}70$ chars) and right-margin safety rules.
2. Update agent system prompts and markdown formatting guidelines.
