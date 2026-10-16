# Study: Bash Here-Doc Appends vs. Native Edit Tools

**Date**: 2026-10-08  
**Scope**: Decision framework, token economics, and ergonomics of bash here-doc (`cat << 'EOF' >> ...`) vs. native structured edit tools (`replace_file_content` / `write_to_file`).

---

## 1. Context & Motivation

Agent loops constantly choose between native structured file-editing tools and shell-based file modification commands. Understanding the exact token overhead, failure modes, and ergonomic trade-offs allows agents to pick the optimal method rather than using one blindly for all scenarios.

---

## 2. Token & Latency Economics

| Dimension | `replace_file_content` | `write_to_file` (Append) | Bash Here-Doc (`cat << 'EOF' >>`) |
|---|---|---|---|
| **JSON Schema Overhead** | High (TargetFile, Instruction, Description, StartLine, EndLine, TargetContent, ReplacementContent) | Medium (TargetFile, Overwrite, Append, Description, CodeContent) | Low (Single CommandLine string) |
| **Context Pre-Requisite** | Requires exact anchor lines in context | None (appends to EOF) | None (appends to EOF) |
| **Tool Round-Trip** | Dedicated step | Dedicated step | Can be chained (`cat >> file && git commit`) |
| **Token Cost for 5-Line Append** | ~180–250 tokens | ~120–160 tokens | ~40–60 tokens |

---

## 3. Decision Framework & Guardrails

### When to Use Bash Here-Doc (`cat << 'EOF' >> ...`):
1. **End-of-File Appends**: Appending notes, addenda, or telemetry logs to markdown documents or tickets.
2. **Temporary / Scratch Files**: Writing out subagent prompt files (e.g. `/tmp/prompt-XXX.txt`) to avoid bash string-interpolation bugs (unescaped backticks/quotes).
3. **Chained Atomic Steps**: When immediately staging, committing, or passing the file to a downstream command in the same turn.

### When to Use Native Edit Tools (`replace_file_content`):
1. **Middle-of-File Edits**: Any modification requiring in-place code replacement or refactoring. (Never use `sed -i` or shell text replacement for code).
2. **Safety & Ambiguity Prevention**: Ensuring changes match exact anchor lines and fail safely if the file has drifted.
3. **Language & IDE Hooks**: When linting feedback or diagnostics hooks are attached to file edit events.

---

## 4. Next Steps & Agent Guidance

1. Conduct empirical audit across Claude Code, Codex, and AGY sessions to evaluate current agent behavior.
2. Formulate a short, token-efficient guideline in `docs/practices/` or `docs/lang/Bash.md` so agents apply the pattern consistently.
