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
	MaxStateBytes  int     `json:"max_state_bytes,omitempty"`
}

// CompactReport summarizes the changes made during compaction.
type CompactReport struct {
	OriginalEntries  int     `json:"original_entries"`
	CompactedEntries int     `json:"compacted_entries"`
	OriginalBytes    int64   `json:"original_bytes"`
	CompactedBytes   int64   `json:"compacted_bytes"`
	ReductionRatio   float64 `json:"reduction_ratio"`
	ExcisedTools     int     `json:"excised_tools"`
	TruncatedResults int     `json:"truncated_results"`
	KeptVerbatim     int     `json:"kept_verbatim"`
}

// historyEntry represents an abridged interaction line for decision model context.
type historyEntry struct {
	Role     string `json:"role,omitempty"`
	Text     string `json:"text,omitempty"`
	ToolCall string `json:"tool_call,omitempty"`
}

// DefaultCompactOptions returns the standard compaction options.
func DefaultCompactOptions() CompactOptions {
	return CompactOptions{
		PinRecent:      5,
		Threshold:      0.50,
		TruncateLength: 200,
		MaxStateBytes:  100 * 1024,
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

func computeEntryBytes(e TranscriptEntry) int64 {
	if len(e.Raw) > 0 {
		if b, err := json.Marshal(e.Raw); err == nil {
			return int64(len(b))
		}
	}
	b, _ := json.Marshal(e)
	return int64(len(b))
}

// fitCompactionState constructs a token/byte-bounded history representation
// using multi-stage progressive fitting so large sessions do not exceed model limits.
func fitCompactionState(entries []TranscriptEntry, candidates []toolCandidate, maxBytes int) (map[string]any, map[string]Question) {
	if maxBytes <= 0 {
		maxBytes = 100 * 1024
	}

	var goals []string
	var firstGoal string
	for _, e := range entries {
		if e.Role == "user" && e.Content != "" {
			if firstGoal == "" {
				firstGoal = e.Content
			}
			goals = append(goals, e.Content)
		}
	}
	if len(goals) > 3 {
		goals = goals[len(goals)-3:]
	}
	if firstGoal == "" && len(entries) > 0 {
		firstGoal = entries[0].Content
	}

	questions := make(map[string]Question)
	for candIdx, cand := range candidates {
		callID := cand.callID
		if callID == "" {
			callID = fmt.Sprintf("t%d", candIdx)
		}
		questions[fmt.Sprintf("cand_%d_keep_call", candIdx)] = Question{
			Type:         TypeNoul,
			Instructions: fmt.Sprintf("Tool call %s (%s) should stay in the history: knowing this call was made, with its input, still matters for what the assistant does next", callID, cand.toolName),
			Criteria: map[string]string{
				"true":  "The tool execution is an important milestone or state change needed for reference",
				"false": "The tool call is intermediate or obsolete noise",
			},
		}
		questions[fmt.Sprintf("cand_%d_keep_result", candIdx)] = Question{
			Type:         TypeNoul,
			Instructions: fmt.Sprintf("The full output of tool call %s (%s, %d bytes) should stay in the history verbatim: the assistant still needs its contents and re-running the tool would not do", callID, cand.toolName, cand.outputLen),
			Criteria: map[string]string{
				"true":  "Verbatim content contains critical errors, code snippets, or definitions",
				"false": "Result is bulky listing, repetitive logs, or output that can be safely truncated",
			},
		}
	}

	for stage := 1; stage <= 6; stage++ {
		curGoal := firstGoal
		curGoals := goals
		if stage >= 2 && len([]rune(curGoal)) > 400 {
			curGoal = string([]rune(curGoal)[:400]) + "..."
		}
		if stage >= 3 && len([]rune(curGoal)) > 150 {
			curGoal = string([]rune(curGoal)[:150]) + "..."
			curGoals = []string{curGoal}
		}

		candidateStates := make(map[string]any)
		for candIdx, cand := range candidates {
			candKey := fmt.Sprintf("candidate_%d", candIdx)
			inputVal := cand.toolInput
			if stage >= 2 && inputVal != nil {
				s := fmt.Sprintf("%v", inputVal)
				limit := 200
				if stage >= 3 {
					limit = 60
				}
				if stage >= 5 {
					limit = 20
				}
				if len([]rune(s)) > limit {
					inputVal = string([]rune(s)[:limit]) + "..."
				}
			}
			if stage >= 6 {
				inputVal = nil
			}
			candidateStates[candKey] = map[string]any{
				"tool_name":     cand.toolName,
				"tool_input":    inputVal,
				"output_length": cand.outputLen,
				"call_id":       cand.callID,
			}
		}

		var history []historyEntry
		var conversationContext []map[string]string
		if stage < 5 {
			for _, e := range entries {
				text := e.Content
				if stage >= 3 && len([]rune(text)) > 400 {
					runes := []rune(text)
					text = string(runes[:250]) + " ... [abridged] ... " + string(runes[len(runes)-100:])
				} else if stage >= 4 && len([]rune(text)) > 150 {
					text = string([]rune(text)[:150]) + "..."
				}

				toolCallSummary := ""
				if e.ToolName != "" || e.ToolCallID != "" {
					outBytes := len(e.ToolOutput)
					if outBytes == 0 {
						outBytes = len(e.Content)
					}
					toolCallSummary = fmt.Sprintf("%s (%s) -> ok, %d chars (omitted)", e.ToolCallID, e.ToolName, outBytes)
				}

				history = append(history, historyEntry{
					Role:     e.Role,
					Text:     text,
					ToolCall: toolCallSummary,
				})
				if (e.Role == "user" || e.Role == "assistant" || e.Role == "system") && stage < 4 {
					conversationContext = append(conversationContext, map[string]string{
						"role":    e.Role,
						"content": text,
					})
				}
			}
		}

		state := map[string]any{
			"initial_instruction": curGoal,
			"initial_goal":        curGoal,
			"recent_goals":        curGoals,
			"candidates":          candidateStates,
			"total_candidates":    len(candidates),
		}
		if len(conversationContext) > 0 {
			state["conversation_context"] = conversationContext
		}
		if len(history) > 0 {
			state["history"] = history
		}

		if b, err := json.Marshal(state); err == nil && len(b) <= maxBytes {
			return state, questions
		}
	}

	// Strictly bounded minimal fallback
	candidateStates := make(map[string]any)
	for candIdx, cand := range candidates {
		candidateStates[fmt.Sprintf("cand_%d", candIdx)] = map[string]any{
			"tool": cand.toolName,
			"len":  cand.outputLen,
		}
	}
	minimalGoal := firstGoal
	if len([]rune(minimalGoal)) > 100 {
		minimalGoal = string([]rune(minimalGoal)[:100]) + "..."
	}
	state := map[string]any{
		"initial_instruction": minimalGoal,
		"candidates":          candidateStates,
		"total_candidates":    len(candidates),
	}
	if b, err := json.Marshal(state); err == nil && len(b) > maxBytes {
		state = map[string]any{
			"initial_instruction": minimalGoal,
			"total_candidates":    len(candidates),
		}
		if b, err := json.Marshal(state); err == nil && len(b) > maxBytes {
			state = map[string]any{
				"total_candidates": len(candidates),
			}
			if b, err := json.Marshal(state); err == nil && len(b) > maxBytes {
				state = map[string]any{}
			}
		}
	}
	return state, questions
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
	if opts.MaxStateBytes <= 0 {
		opts.MaxStateBytes = 100 * 1024
	}

	report := &CompactReport{
		OriginalEntries: len(entries),
	}

	for _, e := range entries {
		report.OriginalBytes += computeEntryBytes(e)
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
		state, questions := fitCompactionState(entries, candidates, opts.MaxStateBytes)
		req := &Request{
			Model:     opts.Model,
			State:     state,
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
				markerTemplate := "\n... [harnez compact truncated %d chars of this tool result; re-run the tool if needed]"
				sampleMarker := fmt.Sprintf(markerTemplate, len(runes))
				markerLen := len([]rune(sampleMarker))

				var truncatedText string
				if opts.TruncateLength <= markerLen {
					shortMarkerTemplate := "\n... [truncated %d chars]"
					sampleShort := fmt.Sprintf(shortMarkerTemplate, len(runes))
					shortLen := len([]rune(sampleShort))
					if opts.TruncateLength > shortLen {
						availRunes := opts.TruncateLength - shortLen
						omitted := len(runes) - availRunes
						truncatedText = string(runes[:availRunes]) + fmt.Sprintf(shortMarkerTemplate, omitted)
					} else {
						truncatedText = string(runes[:opts.TruncateLength])
					}
				} else {
					availRunes := opts.TruncateLength - markerLen
					omitted := len(runes) - availRunes
					truncatedText = string(runes[:availRunes]) + fmt.Sprintf(markerTemplate, omitted)
				}
				// Guarantee strictly bounded by TruncateLength
				if len([]rune(truncatedText)) > opts.TruncateLength {
					truncatedText = string([]rune(truncatedText)[:opts.TruncateLength])
				}

				if entry.ToolOutput != "" {
					entry.ToolOutput = truncatedText
					if len(entry.Raw) > 0 {
						if _, ok := entry.Raw["tool_output"]; ok {
							entry.Raw["tool_output"] = truncatedText
						} else if _, ok := entry.Raw["output"]; ok {
							entry.Raw["output"] = truncatedText
						}
					}
				}
				if entry.Content != "" {
					entry.Content = truncatedText
					if len(entry.Raw) > 0 {
						if _, ok := entry.Raw["content"]; ok {
							entry.Raw["content"] = truncatedText
						}
					}
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
	for _, e := range compacted {
		report.CompactedBytes += computeEntryBytes(e)
	}
	if report.OriginalBytes > 0 {
		report.ReductionRatio = float64(report.OriginalBytes-report.CompactedBytes) / float64(report.OriginalBytes)
		if report.ReductionRatio < 0 {
			report.ReductionRatio = 0
		}
	}

	return compacted, report, nil
}
