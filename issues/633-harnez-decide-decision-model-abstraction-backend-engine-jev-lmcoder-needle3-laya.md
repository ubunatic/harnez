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

Furthermore, harnez plans to support alternative decision backends including **lmcoder** (local/custom embedding or classification setup), **Needle3**, and **Laya**.
We need a unified command `harnez decide` and internal Go engine `internal/decide` to run standalone decisions, filter inputs, and serve as the backbone for skills, hooks, and MCP tools.

## 2. Technical Specification / Findings

### CLI Interface
```bash
# Direct single-question noul check
harnez decide --model jev --noul "Is this command destructive?" "rm -rf build"

# Choice selection from options
harnez decide --model jev --choice "bug_report,feature_request,question,other" "$ISSUE_TEXT"

# Score on a rubric
harnez decide --model jev --score "1:Low,2:Medium,3:High,4:Critical" "$TEXT"

# JSON input/output mode (compatible with Jev SystemOne schema)
harnez decide --model jev --json < payload.json
```

Flags:
- `--model` (default from `~/.harnez/config.yaml` `decide.default_model` or env `HARNEZ_DECIDE_MODEL`, fallback `jev`)
- `--backend`: `jev` (TypeSafe API / Vercel AI Gateway / OpenRouter), `lmcoder` (local embedding/classifier service), `needle3`, `laya`
- `--threshold` (float, for exit code gating e.g. `--threshold 0.8` exits 0 if probability >= 0.8, else 1)
- `--format`: `text`, `json`, `quiet` (for shell script conditionals: `if harnez decide --quiet --threshold 0.8 ...; then ...`)

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
    State     any                 `json:"state"`
    Questions map[string]Question `json:"questions"`
}

type Question struct {
    Type     QuestionType      `json:"type"`
    Question string            `json:"question"`
    Criteria map[string]string `json:"criteria,omitempty"` // for noul true/false or choices
    Levels   []string          `json:"levels,omitempty"`   // for score
}

type DecisionResponse struct {
    Answers map[string]Answer `json:"answers"`
    Latency time.Duration     `json:"latency"`
    CostUSD float64           `json:"cost_usd,omitempty"`
}
```

### Provider Configurations
Configured in `~/.harnez/config.yaml`:
```yaml
decide:
  default_model: jev
  providers:
    jev:
      provider: vercel_ai_gateway # or openrouter, typesafe
      api_key_env: VERCEL_AI_GATEWAY_KEY # fallback OPENROUTER_API_KEY, TYPESAFE_API_KEY
      endpoint: https://ai-gateway.vercel.com/v1/system-one
    lmcoder:
      endpoint: http://127.0.0.1:8088/decide
    laya:
      endpoint: ...
    needle3:
      endpoint: ...
```

## 3. Implementation & Verification Plan
1. **Model Adapter**: Implement `internal/decide/jev.go` handling the HTTP POST request/response for Jev via Vercel AI Gateway and OpenRouter, with offline deterministic mock for tests.
2. **Local / Embedding Adapter**: Implement `internal/decide/lmcoder.go` stub / protocol adapter.
3. **CLI Command**: Add `cmd/harnez/decide.go` exposing `harnez decide` with flags `--model`, `--noul`, `--choice`, `--score`, `--threshold`, `--json`.
4. **Verification**:
   - Unit tests on request formatting, token budget packing, and response parsing.
   - Deterministic offline mock runner in `internal/decide/mock.go`.
   - Integration smoke test with simulated inputs (destructive bash command check, issue triage routing).
