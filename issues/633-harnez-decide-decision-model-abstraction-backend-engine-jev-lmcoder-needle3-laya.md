# 633 — harnez decide: decision-model abstraction & backend engine (Jev, Lmcoder, Needle3, Laya)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: cmd/harnez/, internal/decide/, docs/CLIDesign.md, issues/634, issues/635

---

## 1. Problem & Motivation
Agent workflows require hundreds of low-latency, deterministic judgment calls per session (e.g. classification, rule checking, urgency gating, code review risk, ranking files, and deciding whether to keep tool outputs). 
Using high-end autoregressive chat models (Claude 3.7/Sonnet, GPT-4o) for simple boolean or multiple-choice questions is slow (~2-5s) and orders of magnitude more expensive.
Regex or keyword rules are brittle and fail to capture semantic intent.

Specialized "System 1" non-autoregressive decision models like **Jev** (TypeSafe AI, available via Vercel AI Gateway / OpenRouter / direct) specialize strictly in typed decisions:
- `noul`: binary yes/no probability ($0.0 \dots 1.0$)
- `choice`: single pick from up to 255 defined options with per-choice probabilities and confidence
- `score`: continuous position on a 2-10 level rubric with legend and confidence

Crucially, **Jev is programmed through rich structured JSON**, not simple prompt strings. The wire contract accepts arbitrary structured `state` (objects, arrays, transcripts, code blocks) and detailed per-option `criteria` (instructions and descriptions for each choice and noul pole).

Furthermore, harnez plans to support alternative decision backends including **lmcoder** (local/custom embedding or classification setup), **Needle3**, and **Laya**.
We need a unified command `harnez decide`, declarative spec support (JSON/YAML), and an internal Go engine `internal/decide` to run standalone decisions, filter inputs, and serve as the backbone for skills, hooks, and MCP tools.

## 2. Technical Specification / Findings

### Wire Protocol Contract (Jev SystemOne)
```json
{
  "model": "jev-latest",
  "state": {
    "command": "git push origin main --force",
    "cwd": "/home/uwe/projects/harnez",
    "branch": "main"
  },
  "questions": {
    "action_risk": {
      "type": "choice",
      "instructions": "Classify the operational risk of running `command` in `cwd`",
      "criteria": {
        "read_only": "Commands that only inspect state without mutating disk or git history (ls, git status, cat)",
        "reversible": "Commands that create or modify files but can be cleanly undone or stashed (touch, go test, npm build)",
        "destructive": "Commands that overwrite history, delete untracked files, or affect remote branches (git push -f, rm -rf, git reset --hard)"
      }
    },
    "blocked": {
      "type": "noul",
      "instructions": "Does this require explicit human confirmation?",
      "criteria": {
        "true": "High risk of data loss, destructive action, or production credential mutation",
        "false": "Safe standard developer command"
      }
    }
  }
}
```

### CLI & Declarative Interface
1. **Interactive / Pipeline Mode (JSON / YAML specs)**:
   Supports both stdin and files (`.json` or `.yaml`):
   ```bash
   # Run against a declarative spec file
   harnez decide -f review-gate.yaml --model jev

   # Pipe state JSON with inline question spec
   cat state.json | harnez decide -f spec.yaml

   # Direct JSON stdin
   harnez decide --json < decision_request.json
   ```

2. **Convenient CLI Shortcuts with Rich Criteria**:
   For quick scripts and hooks without full JSON files, support rich criteria via key=value definitions:
   ```bash
   # Noul with explicit criteria
   harnez decide --noul "Is this command destructive?" \
     --criteria "true=Overwrites git history or deletes files,false=Normal read or build" \
     --state '{"cmd": "rm -rf build"}'

   # Choice with rich criteria per option
   harnez decide --choice "risk" \
     --option "safe: Read-only query or lint" \
     --option "review: Modifies core files or schemas" \
     --option "danger: Irreversible delete or remote force push" \
     --state-file diff.patch

   # Score with rubric levels
   harnez decide --score "How severe is this issue?" \
     --level "Cosmetic only" \
     --level "Degraded with workaround" \
     --level "Critical blocker with no workaround" \
     "$ISSUE_TEXT"
   ```

3. **Output & Gating Flags**:
   - `--threshold <float>`: for shell script conditionals. Exits 0 if probability/confidence >= threshold, else exits 1.
   - `--pick <question_id>`: outputs only the raw answer (e.g. `danger` or `0.94`) for simple shell assignment `ACTION=$(harnez decide ... --pick risk)`.
   - `--format`: `json` (full response), `table` (summary table with latencies & probabilities), `quiet` (exit code only).

### Backend Abstraction
In `internal/decide/`:
```go
type Backend interface {
    Decide(ctx context.Context, req *DecisionRequest) (*DecisionResponse, error)
}

type QuestionType string
const (
    QuestionTypeNoul   QuestionType = "noul"
    QuestionTypeChoice QuestionType = "choice"
    QuestionTypeScore  QuestionType = "score"
)

type DecisionRequest struct {
    Model     string               `json:"model,omitempty" yaml:"model,omitempty"`
    State     any                  `json:"state" yaml:"state"`
    Questions map[string]Question  `json:"questions" yaml:"questions"`
}

type Question struct {
    Type         QuestionType `json:"type" yaml:"type"`
    Instructions string       `json:"instructions" yaml:"instructions"`
    // Criteria:
    // For noul: {"true": "...", "false": "..."}
    // For choice: {"option_a": "description...", "option_b": "description..."}
    // For score: []string{"level 0 desc", "level 1 desc", ...}
    Criteria any `json:"criteria,omitempty" yaml:"criteria,omitempty"`
}

type NoulAnswer struct {
    Type NoulType `json:"type"`
    Noul float64  `json:"noul"`
}

type ChoiceAnswer struct {
    Type          string             `json:"type"`
    Choice        string             `json:"choice"`
    Probabilities map[string]float64 `json:"probabilities"`
    Confidence    float64            `json:"confidence"`
}

type ScoreAnswer struct {
    Type          string             `json:"type"`
    Score         float64            `json:"score"`
    Legend        map[string]string  `json:"legend"`
    Probabilities map[string]float64 `json:"probabilities"`
    Confidence    float64            `json:"confidence"`
}

type DecisionResponse struct {
    Model   string            `json:"model"`
    Answers map[string]any    `json:"answers"` // unmarshals into NoulAnswer, ChoiceAnswer, ScoreAnswer
    Usage   DecisionUsage     `json:"usage"`
    Latency time.Duration     `json:"latency"`
}
```

### Provider Configurations
Configured in `~/.harnez/config.yaml`:
```yaml
decide:
  default_model: jev
  providers:
    jev:
      provider: typesafe # or vercel_ai_gateway, openrouter
      api_key_env: TYPESAFE_API_KEY # user key, set in ~/.userrc; fallback VERCEL_AI_GATEWAY_KEY, OPENROUTER_API_KEY
      endpoint: https://ai-gateway.vercel.com/v1/system-one # gateway only; verify the direct TypeSafe endpoint
    lmcoder:
      endpoint: http://127.0.0.1:8088/decide
    laya:
      endpoint: https://api.laya.ai/v1/decisions
    needle3:
      endpoint: https://api.needle3.ai/v1/decisions
```

## 3. Progress (2026-09-29)
Built the first slice; see `docs/Decide.md`:
- `internal/decide`: `Backend` interface, request/answer types, spec loader, `systemone` client.
- `spec/decide.yaml` + schema: backend `jev` (TypeSafe direct, `$TYPESAFE_API_KEY`). A local
  model plugs in as a spec entry (same protocol) or as a new protocol client.
- `harnez decide`: `-f`, `--noul/--choice/--score`, `--option`, `--level`, state from flags,
  args or stdin, `--pick`, `--threshold`, `--format table|json|quiet`. Verified live.

Not done yet: user config overrides in `~/.harnez/config.yaml` (only the embedded spec and env
vars today), batching several requests in parallel, a mock backend beyond the test server,
lmcoder/Needle3/Laya entries (no endpoints known yet).

## 4. Implementation & Verification Plan
1. **Model Adapter**: Implement `internal/decide/jev.go` handling the HTTP POST request/response for Jev via Vercel AI Gateway, OpenRouter, and TypeSafe, with offline deterministic mock for tests.
2. **Local / Embedding Adapter**: Implement `internal/decide/lmcoder.go` stub / protocol adapter.
3. **CLI Command**: Add `cmd/harnez/decide.go` supporting structured file inputs (`-f`, `--state-file`, `--json`), multi-option `--option "name: desc"` flags, `--threshold`, and `--pick`.
4. **Verification**:
   - Unit tests on request formatting, token budget packing, and response parsing.
   - Deterministic offline mock runner in `internal/decide/mock.go`.
   - Integration smoke test with rich criteria (destructive bash command check, issue triage routing, code review risk rubric).
