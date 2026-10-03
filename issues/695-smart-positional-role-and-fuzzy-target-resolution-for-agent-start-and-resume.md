# 695 — Smart positional role and fuzzy target resolution for agent start and resume

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 686 (role resolution & inference), 692 (static role aliases)

---

## 1. Problem & Motivation
Currently:
- `harnez agent start` requires flags like `--role <name>` and `--name <name>` to configure basic session properties.
- `harnez agent resume` requires explicit UUID prefixes or exact `--name` matches, rather than fuzzy session matching or contextual resume by working directory.

## 2. Technical Specification & Ergonomics

### A. Positional Arguments in `agent start`
Allow leading positional tokens to set the role or session name without typing flags:
```bash
# Leading token matches role or alias -> sets role
harnez agent start explorer "audit SQLite queries in neus"
harnez agent start review "check diff on HEAD~1"

# Positional role + session name shorthand:
harnez agent start coder my-worker "implement ticket 041"
```

### B. Fuzzy & Contextual Target Resolution in `agent resume`
1. **Fuzzy Name Resolution**:
   ```bash
   # Matches 'neus-classify-dev' from substring
   harnez agent resume classify "run tests again"
   ```
2. **Contextual Resume (No ID/Name Provided)**:
   ```bash
   # Automatically attaches to the latest active session in the current repo/dir
   cd /home/uwe/projects/neus
   harnez agent resume "continue with M2"
   ```

## 3. Implementation & Verification Plan
- Parse leading positional arguments in `cmd/harnez/agent.go` and `cmd/harnez/agent_run.go`.
- If `arg[0]` matches a canonical role or alias (`subagent.ResolveRole(arg[0])`), consume it as the role.
- If `arg[0]` matches a session in the store (or a unique substring of an active session), resolve to that session.
- If `resume` is called with prompt text but no target ID, resolve to the most recent session associated with current working directory (`$PWD`).
- Unit and CLI tests in `cmd/harnez/agent_test.go`.
