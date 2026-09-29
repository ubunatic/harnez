// Package decide implements verbatim transcript compaction powered by fast
// decision models (issue 635).
package decide

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// TranscriptEntry represents one message or tool interaction in an agent transcript.
type TranscriptEntry struct {
	Index      int            `json:"index,omitempty"`
	Role       string         `json:"role,omitempty"` // "system", "user", "assistant", "tool"
	Type       string         `json:"type,omitempty"`
	Content    string         `json:"content,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
	ToolInput  any            `json:"tool_input,omitempty"`
	ToolOutput string         `json:"tool_output,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

// CompactOptions controls transcript compaction behavior.
type CompactOptions struct {
	Model          string  `json:"model,omitempty"`
	PinRecent      int     `json:"pin_recent"`
	Threshold      float64 `json:"threshold"`
	TruncateLength int     `json:"truncate_length"`
}

// CompactReport summarizes the changes made during compaction.
type CompactReport struct {
	OriginalEntries  int `json:"original_entries"`
	CompactedEntries int `json:"compacted_entries"`
	ExcisedTools     int `json:"excised_tools"`
	TruncatedResults int `json:"truncated_results"`
	KeptVerbatim     int `json:"kept_verbatim"`
}

// DefaultCompactOptions returns the standard compaction options.
func DefaultCompactOptions() CompactOptions {
	return CompactOptions{
		PinRecent:      5,
		Threshold:      0.50,
		TruncateLength: 200,
	}
}

// ParseJSONLTranscript reads JSONL transcript lines into TranscriptEntry slice.
func ParseJSONLTranscript(r io.Reader) ([]TranscriptEntry, error) {
	var entries []TranscriptEntry
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // up to 10MB line buffer

	lineNum := 0
	entryIdx := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, fmt.Errorf("line %d: invalid JSON: %w", lineNum, err)
		}

		entry := TranscriptEntry{
			Index: entryIdx,
			Raw:   raw,
		}

		if r, ok := raw["role"].(string); ok {
			entry.Role = r
		}
		if t, ok := raw["type"].(string); ok {
			entry.Type = t
		}
		if c, ok := raw["content"].(string); ok {
			entry.Content = c
		}
		if tn, ok := raw["tool_name"].(string); ok {
			entry.ToolName = tn
		}
		if ti, ok := raw["tool_input"]; ok {
			entry.ToolInput = ti
		}
		if to, ok := raw["tool_output"].(string); ok {
			entry.ToolOutput = to
		} else if out, ok := raw["output"].(string); ok {
			entry.ToolOutput = out
		}
		if tcid, ok := raw["tool_call_id"].(string); ok {
			entry.ToolCallID = tcid
		} else if id, ok := raw["tool_use_id"].(string); ok {
			entry.ToolCallID = id
		} else if id, ok := raw["id"].(string); ok && (entry.Role == "tool" || entry.Type == "tool_use" || entry.Type == "tool_result") {
			entry.ToolCallID = id
		}

		// Also detect nested tool calls in raw (OpenAI / Claude schemas)
		if entry.ToolCallID == "" {
			if tcList, ok := raw["tool_calls"].([]any); ok && len(tcList) > 0 {
				if firstTc, ok := tcList[0].(map[string]any); ok {
					if id, ok := firstTc["id"].(string); ok {
						entry.ToolCallID = id
					}
					if fn, ok := firstTc["function"].(map[string]any); ok {
						if name, ok := fn["name"].(string); ok && entry.ToolName == "" {
							entry.ToolName = name
						}
						if args, ok := fn["arguments"]; ok && entry.ToolInput == nil {
							entry.ToolInput = args
						}
					}
				}
			}
		}

		if entry.Role == "" && (entry.ToolName != "" || entry.ToolOutput != "") {
			entry.Role = "tool"
		}

		entries = append(entries, entry)
		entryIdx++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read transcript: %w", err)
	}
	return entries, nil
}

// WriteJSONLTranscript writes compacted entries to an output writer.
func WriteJSONLTranscript(w io.Writer, entries []TranscriptEntry) error {
	enc := json.NewEncoder(w)
	for _, e := range entries {
		if len(e.Raw) > 0 {
			if e.ToolOutput != "" {
				if _, ok := e.Raw["tool_output"]; ok {
					e.Raw["tool_output"] = e.ToolOutput
				} else if _, ok := e.Raw["output"]; ok {
					e.Raw["output"] = e.ToolOutput
				}
			}
			if e.Content != "" {
				if _, ok := e.Raw["content"]; ok {
					e.Raw["content"] = e.Content
				}
			}
			if err := enc.Encode(e.Raw); err != nil {
				return err
			}
		} else {
			if err := enc.Encode(e); err != nil {
				return err
			}
		}
	}
	return nil
}

// toolCandidate holds candidate information for a tool call or result to be evaluated.
type toolCandidate struct {
	callIdx   int
	resultIdx int
	toolName  string
	toolInput any
	outputLen int
	callID    string
}

// CompactTranscript performs decision-model-driven verbatim transcript pruning.
func CompactTranscript(ctx context.Context, backend Backend, entries []TranscriptEntry, opts CompactOptions) ([]TranscriptEntry, *CompactReport, error) {
	if opts.PinRecent <= 0 {
		opts.PinRecent = 5
	}
	if opts.Threshold < 0 {
		opts.Threshold = 0.50
	}
	if opts.TruncateLength <= 0 {
		opts.TruncateLength = 200
	}

	report := &CompactReport{
		OriginalEntries: len(entries),
	}

	if len(entries) == 0 {
		report.CompactedEntries = 0
		return entries, report, nil
	}

	recentThreshold := len(entries) - opts.PinRecent

	// 1. Identify paired tool interactions and unpaired tool calls/results
	callMap := make(map[string]int)
	resultMap := make(map[string]int)

	for i, e := range entries {
		if e.ToolCallID != "" {
			if e.Role == "tool" || e.ToolOutput != "" || e.Type == "tool_result" {
				resultMap[e.ToolCallID] = i
			} else {
				callMap[e.ToolCallID] = i
			}
		}
	}

	// 2. Identify candidates eligible for pruning (both call and result must be outside the pinned range)
	var candidates []toolCandidate
	candidateSet := make(map[int]bool)

	for i, entry := range entries {
		if i == 0 || i >= recentThreshold {
			continue // Pinned
		}

		if candidateSet[i] {
			continue
		}

		// Check if this entry is a tool call or tool result
		if entry.ToolCallID != "" {
			callIdx := -1
			resultIdx := -1
			toolName := entry.ToolName
			toolInput := entry.ToolInput
			outputLen := len(entry.ToolOutput)

			if cIdx, ok := callMap[entry.ToolCallID]; ok {
				callIdx = cIdx
				if toolName == "" {
					toolName = entries[cIdx].ToolName
				}
				if toolInput == nil {
					toolInput = entries[cIdx].ToolInput
				}
			}
			if rIdx, ok := resultMap[entry.ToolCallID]; ok {
				resultIdx = rIdx
				outputLen = len(entries[rIdx].ToolOutput)
				if outputLen == 0 {
					outputLen = len(entries[rIdx].Content)
				}
			}

			// Invariant: If either part of the pair is pinned (index == 0 or >= recentThreshold),
			// the entire interaction is protected and must NOT be pruned.
			if (callIdx >= 0 && (callIdx == 0 || callIdx >= recentThreshold)) ||
				(resultIdx >= 0 && (resultIdx == 0 || resultIdx >= recentThreshold)) {
				continue
			}

			if callIdx >= 0 || resultIdx >= 0 {
				if callIdx >= 0 {
					candidateSet[callIdx] = true
				}
				if resultIdx >= 0 {
					candidateSet[resultIdx] = true
				}
				candidates = append(candidates, toolCandidate{
					callIdx:   callIdx,
					resultIdx: resultIdx,
					toolName:  toolName,
					toolInput: toolInput,
					outputLen: outputLen,
					callID:    entry.ToolCallID,
				})
				continue
			}
		}

		// Single unlinked tool entry
		if entry.Role == "tool" || entry.ToolName != "" || entry.ToolOutput != "" {
			candidateSet[i] = true
			outputLen := len(entry.ToolOutput)
			if outputLen == 0 {
				outputLen = len(entry.Content)
			}
			candidates = append(candidates, toolCandidate{
				callIdx:   i,
				resultIdx: i,
				toolName:  entry.ToolName,
				toolInput: entry.ToolInput,
				outputLen: outputLen,
			})
		}
	}

	// 3. Batch all questions into parallel/batch decision queries
	type decisionResult struct {
		keepCall   bool
		keepResult bool
	}
	decisions := make(map[int]decisionResult)

	if len(candidates) > 0 {
		questions := make(map[string]Question)
		candidateStates := make(map[string]any)

		for candIdx, cand := range candidates {
			candKey := fmt.Sprintf("candidate_%d", candIdx)
			candidateStates[candKey] = map[string]any{
				"tool_name":     cand.toolName,
				"tool_input":    cand.toolInput,
				"output_length": cand.outputLen,
				"call_id":       cand.callID,
			}

			questions[fmt.Sprintf("cand_%d_keep_call", candIdx)] = Question{
				Type:         TypeNoul,
				Instructions: fmt.Sprintf("Does the agent's context need to know that tool %q was executed with these arguments?", cand.toolName),
				Criteria: map[string]string{
					"true":  "The tool execution is an important milestone or state change needed for reference",
					"false": "The tool call is intermediate or obsolete noise",
				},
			}
			questions[fmt.Sprintf("cand_%d_keep_result", candIdx)] = Question{
				Type:         TypeNoul,
				Instructions: fmt.Sprintf("Does the verbatim tool output (length %d bytes) still matter for subsequent reasoning?", cand.outputLen),
				Criteria: map[string]string{
					"true":  "Verbatim content contains critical errors, code snippets, or definitions",
					"false": "Result is bulky listing, repetitive logs, or output that can be safely truncated",
				},
			}
		}

		// Extract initial user instructions and abridged conversation messages for decision context
		var initialInstruction string
		var conversationContext []map[string]string
		for _, entry := range entries {
			if entry.Role == "user" && initialInstruction == "" {
				initialInstruction = entry.Content
			}
			if entry.Role == "user" || entry.Role == "assistant" || entry.Role == "system" {
				c := entry.Content
				if len([]rune(c)) > 500 {
					c = string([]rune(c)[:500]) + "..."
				}
				if c != "" {
					conversationContext = append(conversationContext, map[string]string{
						"role":    entry.Role,
						"content": c,
					})
				}
			}
		}
		if initialInstruction == "" && len(entries) > 0 {
			initialInstruction = entries[0].Content
		}

		req := &Request{
			Model: opts.Model,
			State: map[string]any{
				"initial_instruction":  initialInstruction,
				"conversation_context": conversationContext,
				"candidates":           candidateStates,
				"total_candidates":     len(candidates),
			},
			Questions: questions,
		}

		res, err := backend.Decide(ctx, req)
		if err != nil {
			// Fail open: default to keeping all
			for candIdx := range candidates {
				decisions[candIdx] = decisionResult{keepCall: true, keepResult: true}
			}
		} else {
			for candIdx := range candidates {
				kc := true
				kr := true
				if a, ok := res.Answers[fmt.Sprintf("cand_%d_keep_call", candIdx)]; ok && a.Noul != nil {
					kc = *a.Noul >= opts.Threshold
				}
				if a, ok := res.Answers[fmt.Sprintf("cand_%d_keep_result", candIdx)]; ok && a.Noul != nil {
					kr = *a.Noul >= opts.Threshold
				}
				decisions[candIdx] = decisionResult{keepCall: kc, keepResult: kr}
			}
		}
	}

	// 4. Build excise and truncate sets
	exciseIndices := make(map[int]bool)
	truncateIndices := make(map[int]bool)

	for candIdx, cand := range candidates {
		d := decisions[candIdx]
		if !d.keepCall && !d.keepResult {
			// Excise call and result
			if cand.callIdx >= 0 {
				exciseIndices[cand.callIdx] = true
			}
			if cand.resultIdx >= 0 {
				exciseIndices[cand.resultIdx] = true
			}
			report.ExcisedTools++
		} else if d.keepCall && !d.keepResult {
			// Keep call, truncate result
			if cand.resultIdx >= 0 {
				truncateIndices[cand.resultIdx] = true
			}
		}
	}

	// 5. Reconstruct transcript
	var compacted []TranscriptEntry
	for i, entry := range entries {
		if exciseIndices[i] {
			continue // Excised
		}

		if truncateIndices[i] {
			outputContent := entry.ToolOutput
			if outputContent == "" {
				outputContent = entry.Content
			}

			runes := []rune(outputContent)
			if len(runes) > opts.TruncateLength {
				marker := "\n... [truncated by harnez compact]"
				markerRunes := []rune(marker)
				var truncatedText string
				if opts.TruncateLength <= len(markerRunes) {
					truncatedText = string(runes[:opts.TruncateLength])
				} else {
					availRunes := opts.TruncateLength - len(markerRunes)
					truncatedText = string(runes[:availRunes]) + marker
				}
				// Guarantee strictly bounded by TruncateLength
				if len([]rune(truncatedText)) > opts.TruncateLength {
					truncatedText = string([]rune(truncatedText)[:opts.TruncateLength])
				}

				if entry.ToolOutput != "" {
					entry.ToolOutput = truncatedText
				}
				if entry.Content != "" {
					entry.Content = truncatedText
				}
				report.TruncatedResults++
			} else {
				report.KeptVerbatim++
			}
			compacted = append(compacted, entry)
			continue
		}

		compacted = append(compacted, entry)
		report.KeptVerbatim++
	}

	report.CompactedEntries = len(compacted)
	return compacted, report, nil
}
