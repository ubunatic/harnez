<!-- harnez:topic: Eager Global Doc Include Stripping, Startup Token Tax, and Prompt Distribution Architecture -->
# Eager Global Doc Include Stripping, Startup Token Tax, and Prompt Distribution Architecture

**Date**: 2026-09-17
**Scope**: `config.yaml`, `internal/claude/`, `~/.claude/CLAUDE.md`, `~/.prime/agent/AGENTS.md`, `issues/386`
**Tickets**: 386, 354, 312, 311, 385, 387

---

## 1. Executive Summary

This study documents the investigation, resolution, and architectural standardization of prompt-distribution mechanics across global and local instruction templates in `harnez`.

In [Issue 386](file:///home/uwe/projects/harnez/issues/386-strip-eager-global-doc-includes-from-global-claude-md-template.md), we identified that Claude Code (`claude`) treats any `@<path>` reference inside `CLAUDE.md` as an eager macro file inclusion directive. Because `config.yaml` included naked `@docs/AgenticLoop.md` and `@docs/Bash.md` citations inside the global `Instructions Hierarchy` section, running `harnez apply` caused Claude Code to eagerly inline **~12.6 KB (~3,250 tokens)** into **every single session globally across the entire machine** — even in clean, non-harnez repositories or unrelated projects.

We stripped the eager `@` prefix from global templates, transitioning them to lazy/soft reference pointers (`(see docs/Bash.md §8)`, `(AgenticLoop Invariant 6)`), added an automated unit test regression guard in `internal/claude`, verified clean Quota-1 test execution, and audited cross-harness behaviors.

---

## 2. The Mechanism: Claude Code's `@` File-Inclusion Parser

Claude Code features a built-in pre-processor that scans `CLAUDE.md` and prompt inputs for `@path/to/file` tokens:
- When an `@path` token matches an existing file on disk (relative to the instruction file or working directory), Claude Code intercepts the reference and inlines the complete file content directly into the initial system prompt.
- In `config.yaml` (`agents_md.global.sections` -> `Instructions Hierarchy`), two informal citations were originally written using `@docs/...`:
  1. `... see @docs/AgenticLoop.md Invariant 6.`
  2. `... See @docs/Bash.md §8.`
- Because `harnez apply` installs the evergreen documentation library into `~/.claude/docs/`, both target files were guaranteed to exist globally:
  - `~/.claude/docs/AgenticLoop.md`: 7,807 bytes (~2,000 tokens)
  - `~/.claude/docs/Bash.md`: 4,820 bytes (~1,250 tokens)
- Consequently, Claude Code inlined both full documents (~12,627 bytes / ~3,250 tokens) before the agent received turn 1 in any directory on the workstation.

---

## 3. Impact Analysis: Global Startup Tax vs. Minimal Global Docs

The unintended eager inlining caused two major problems:

1. **Massive Global Context Inflation**:
   - Every session in any folder — even an empty scratchpad or Python/Rust repository — was burdened with 3,250+ tokens of Go/Bash/AgenticLoop instructions.
   - For multi-turn conversations, this startup tax multiplied the token consumption and degraded the prompt signal-to-noise ratio.

2. **Violation of the Minimal Global Docs Invariant**:
   - `config.yaml` explicitly establishes the rule:
     > *"Minimal Global Docs: keep this global file free of anything not relevant to every project — it applies everywhere and is glue between local and global docs only, steering the agentic setup rather than carrying project-specific content."*
   - Eager global doc inclusion directly violated this contract by turning a lightweight glue file into a multi-kilobyte documentation payload.

---

## 4. Architectural Principle: Global Lazy Pointers vs. Project-Local Eager Opt-In

To maintain clear boundaries, `harnez` defines a strict dual-layer doc distribution model:

```
                  ┌────────────────────────────────────────────────────────┐
                  │ GLOBAL INSTRUCTIONS (~/.claude/CLAUDE.md, ~/AGENTS.md) │
                  │ - Flat, applies across ALL projects on the machine     │
                  │ - LAZY REFERENCES ONLY: "see docs/Bash.md §8"         │
                  │ - Zero eager @docs/ inclusion triggers                 │
                  └────────────────────────────────────────────────────────┘
                                              │
                                              │ Project Opt-In (harnez init)
                                              ▼
                  ┌────────────────────────────────────────────────────────┐
                  │ LOCAL INSTRUCTIONS (./AGENTS.md, ./CLAUDE.md)          │
                  │ - Scoped strictly to current repository               │
                  │ - EAGER OPT-IN: "- Go/Golang @docs/Go.md"             │
                  │ - Inlines only the specific rules this project needs   │
                  └────────────────────────────────────────────────────────┘
```

- **Global Templates**: Must use lazy, soft pointers (e.g. `(AgenticLoop Invariant 6)` and `(see docs/Bash.md §8)`). Agents that need deep details can inspect `docs/` or `~/.claude/docs/` on demand using tools (`view_file`, `grep_search`).
- **Project-Local Templates**: Project `AGENTS.md` files generated by `harnez init` explicitly opt into languages (e.g. Go, Bash, Make, Rust) and intentionally use `@docs/...` to inline active conventions for that specific workspace.

---

## 5. Automated Regression Prevention

To prevent future edits to `config.yaml` from accidentally reintroducing eager doc includes into global templates, we implemented an automated unit test in [`internal/claude/global_claude_test.go`](file:///home/uwe/projects/harnez/internal/claude/global_claude_test.go):

```go
func TestGlobalClaudeSectionsHaveNoEagerDocIncludes(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded() failed: %v", err)
	}

	eagerDocRegex := regexp.MustCompile(`(?m)(?:^|\s)@docs/\S+`)

	for _, sec := range cfg.AgentsMD.Global.Sections {
		t.Run(sec.Name, func(t *testing.T) {
			if matches := eagerDocRegex.FindAllString(sec.Content, -1); len(matches) > 0 {
				t.Errorf("global section %q contains eager doc includes %v; remove '@' prefix so Claude Code does not inline full doc files globally (issue 386)", sec.Name, matches)
			}
		})
	}
}
```

This test runs on every `make test` / `make test-q1` invocation and CI build, ensuring that global templates remain strictly minimal.

---

## 6. Cross-Harness Comparison

| Agent Harness | Pre-Fix Behavior with `@docs/...` | Post-Fix Behavior with `docs/...` | Lookup Capability |
|---|---|---|---|
| **Claude Code** | Eagerly inlines full file into startup prompt | Treats as soft citation, zero startup token tax | On-demand via `view_file` / `grep` |
| **Antigravity / Gemini CLI** | Treated as plain text reference pointer | Cleaner reference pointer | On-demand via `view_file` / `grep_search` |
| **Codex** | Treated as plain text reference pointer | Clean reference pointer | On-demand via file tools |
| **OpenCode / Pi** | Treated as plain text reference pointer | Clean reference pointer | On-demand via file tools |
| **Prime** | Received unexpanded `@` via `~/.prime/agent/AGENTS.md` | Clean synced reference pointer | On-demand via `./docs` or `~/.prime/agent/docs` |

---

## 7. Lessons & Recommendations

1. **Distinguish Citations from Directives**: In harnesses with macro-expansion syntax (like Claude Code's `@`), never use macro characters for conversational or documentary citations in global templates.
2. **Quantify Prompt Bloat with Real Metrics**: Always measure startup prompt size across clean, empty testbeds to detect silent inclusion bloat early.
3. **Guard Templates with Static Regex Tests**: Configuration templates that generate system prompts should have automated assertions checking for accidental inclusion tokens.
