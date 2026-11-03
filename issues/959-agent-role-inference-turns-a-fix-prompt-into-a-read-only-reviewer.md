# 959 — agent role inference turns a fix prompt into a read-only reviewer

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 958, ubunatic.com 120 (30-minute live-build talk)

---

## 1. Problem & Motivation
`harnez agent --model codex:sol:low -d . -p "A reviewer found: ... Fix it and add boundary
tests ... Run go test ./... and commit."` printed `harnez: inferred role "reviewer" from prompt`.
The agent then refused to edit ("your explicit reviewer role requires read-only inspection"),
although the prompt asked for a fix and a commit. Found while rehearsing the live-build talk;
on stage this would silently skip the fix step.

## 2. Technical Specification / Findings
- The word "reviewer" in the prompt outweighed the imperative verbs "Fix", "add", "commit".
- The agent called the inferred role "explicit", so the inference is not visible as a guess.
- Workaround: pass `--role developer`.

## 3. Implementation & Verification Plan
- Prefer imperative edit verbs (fix, implement, add, commit) over role nouns when inferring.
- Tell the agent the role was inferred, not explicit.
- Test: the prompt above infers developer, or at least warns about the conflict.

## Audit 2026-11-03

Jules audit (PR GH#66, report `reports/jules/959.md`, checked against d8904997, all references held): still open. Inference happens at `internal/agent/core.go:85-88`; `WithProtocol` (`internal/agent/core.go:28-31`) injects `Harnez role: <role>.` with no hint that it was inferred, so the model reads it as explicit. Smallest step: pass an `inferred` flag from `StartRole` into `WithProtocol` and say so in the preamble.
