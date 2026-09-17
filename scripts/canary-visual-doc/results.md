# Visual Doc Behavioral Canary Results

Run via:
```bash
go run ./scripts/canary-visual-doc -baseline docs/lang/Bash.md scratch/vision/Bash_2col.png scripts/canary-visual-doc/fixtures/bash-deploy-check.task.md deploy-check.sh
```
or helper script:
```bash
./scripts/canary-visual-doc/run.sh
```

## 1. Overview & Experimental Design

This canary (Issue 393) evaluates multimodal instruction following:
1. Spawns an isolated subagent in a clean scratch directory outside any harnez project.
2. Auto-injection of `AGENTS.md` / `CLAUDE.md` is strictly absent.
3. Provides **ZERO text rules or text style guides**. The sole style reference is an attached visual cheatsheet card (`STYLE_GUIDE.png`).
4. Executes real coding tasks (e.g. `bash-deploy-check.task.md`).
5. Mechanically lints the generated output file with `internal/lint` (`lint.DefaultLinter().LintBytes(...)`) without subjective LLM scoring.

---

## 2. Multi-Harness ViT Token & Compression Summary

| Cheatsheet Asset | Geometry (px) | Baseline Text Tokens | OpenAI / Codex Tokens | OpenAI Savings | Gemini Tokens | Gemini Savings | Claude Tokens | Claude Savings |
|---|---|---|---|---|---|---|---|---|
| `Bash_2col.png` | 1200 × 2004 | 2,611 | 1,105 | **2.36x** | 1,548 | **1.69x** | 3,207 | 0.81x |
| `Bash_3col.png` | 1408 × 1262 | 2,611 | 765 | **3.41x** | 1,032 | **2.53x** | 2,370 | **1.10x** |
| `dev_cheatsheet_3in1.png` | 1440 × 1400 | 6,473 | 765 | **8.46x** | 1,032 | **6.27x** | 2,688 | **2.41x** |

---

## 3. Canary Execution & Lint Results

| Fixture | Input Cheatsheet | Output Target | Mechanical Lint Findings | Result |
|---|---|---|---|---|
| `bash-deploy-check.task.md` | `scratch/vision/Bash_2col.png` | `deploy-check.sh` | 0 findings (`if test`, `git -C`, `trap`, `local` valid) | **PASS** |
| `bash-deploy-check.task.md` | `scratch/vision/dev_cheatsheet_3in1.png` | `deploy-check.sh` | 0 findings | **PASS** |

### Verified Invariants Checked Mechanically:
- Strict mode header: `set -euo pipefail`
- POSIX test conditionals: `if test "$#"` and `if test ! -d` instead of `[[ ... ]]` or `[ ... ]`
- No trailing semicolons before keywords (`if test ...; then` vs newline before `then`)
- Scoped repository commands: `git -C "$dir" status` rather than `cd "$dir"`
- Clean resource cleanup: `mktemp` and `trap 'rm -f ...' EXIT`
