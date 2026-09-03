# 216 — Switch Telemetry Note Classifier from Claude CLI to Local SLM via `lmcoder`

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature / Telemetry
**Related**: [212-taxonomic-classification-of-telemetry-tool-notes.md](212-taxonomic-classification-of-telemetry-tool-notes.md), [204-sanitized-telemetry-and-token-export-for-datavis.md](204-sanitized-telemetry-and-token-export-for-datavis.md), `internal/telemetry/classify.go`, `cmd/harnez/usageexport.go`

---

## 1. Problem & Motivation

In issue 212, `harnez usage export --classify` added taxonomic classification of telemetry tool notes into closed enum categories (`test`, `build`, `edit`, `inspection`, `git`, `debug`, `workflow`, `config`, `other`).

However, its fallback `DefaultLocalClassifier` implementation in `internal/telemetry/classify.go` currently executes:
```go
cmd := exec.CommandContext(cctx, "claude", "-p", "--output-format", "json")
```
`claude -p` is **not** a local LLM — it is a cloud-hosted frontier model. 

Using `claude -p` for classification has three drawbacks:
1. **Unnecessary Cloud Round-Trip & Cost**: Classifying short notes into 9 fixed enums is a trivial NLP task that does not justify paying cloud token rates or waiting on cloud latency.
2. **Privacy Exposure**: Sending un-sanitized prose to Anthropic's cloud endpoints conflicts with the goal of keeping internal notes strictly local on the developer workstation.
3. **Existing Local Tooling**: The workstation already has [`lmcoder`](/lmcoder) installed (`/home/uwe/go/bin/lmcoder`) with cached lightweight models (`qwen2.5-0.5b-instruct-q4`, `qwen2.5-3b-instruct-q4`, `qwen3-4b-instruct-2507-q4`) ready to serve locally on the AMD Cezanne GPU.

---

## 2. Proposed Architecture & Solution

### 2.1 Native `lmcoder` / OpenAI-Compatible Local Endpoint
Update `internal/telemetry/classify.go` to invoke local model serving via `lmcoder`:

1. **Local Endpoint Discovery**:
   - Check if an OpenAI-compatible local completion server is already running (e.g. `http://localhost:8080/v1` or `lmcoder` proxy/serve endpoint).
   - Alternatively, support invoking `lmcoder` or a local binary directly via stdin/stdout or curl/HTTP.
2. **On-Demand Local Model Serving**:
   - If `lmcoder status` shows not serving, support a headless single-turn invocation or ephemeral start/stop:
     - `lmcoder serve --model=qwen2.5-0.5b-instruct-q4` (or small 3B/4B model).
3. **Remove Cloud Default**:
   - Remove `bin = "claude"` as the default classifier.
   - If no local model runner is available or running, `harnez usage export --classify` should either:
     - Fall back safely to Tier 1 regexes (`CategoryOther` for unmatched) with an informative warning: `local model runner not detected; install or start lmcoder to classify unmatched notes`.
     - Never silently call out to external cloud APIs under a `--classify` flag that promises local classification.

---

## 3. Acceptance Criteria

1. `DefaultLocalClassifier` in `internal/telemetry/classify.go` targets a local runner (`lmcoder` / localhost HTTP endpoint), not `claude -p`.
2. Free-form note text is never transmitted across the network or to third-party cloud APIs during classification.
3. Unmatched notes safely fall back to `CategoryOther` if `lmcoder` is stopped, without crashing or stalling the export.
4. Unit tests mock the local HTTP/CLI response and verify valid JSON enum parsing.
