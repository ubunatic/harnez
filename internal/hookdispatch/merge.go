package hookdispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// claudeReply is the part of a Claude hook reply that the merge models. Its
// hookSpecificOutput is merged field by field; systemMessage is joined. Every
// other top-level key is kept first-wins (see mergeClaudeReplies).
type claudeReply struct {
	HookSpecificOutput *specificOutput `json:"hookSpecificOutput,omitempty"`
	SystemMessage      string          `json:"systemMessage,omitempty"`
}

type specificOutput struct {
	HookEventName            string          `json:"hookEventName,omitempty"`
	PermissionDecision       string          `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string          `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             json.RawMessage `json:"updatedInput,omitempty"`
	AdditionalContext        string          `json:"additionalContext,omitempty"`
}

func (s specificOutput) empty() bool {
	return s.HookEventName == "" && s.PermissionDecision == "" && s.PermissionDecisionReason == "" &&
		!isRewrite(s.UpdatedInput) && s.AdditionalContext == ""
}

// isRewrite reports whether an updatedInput value rewrites the tool input. An
// absent or JSON null value does not.
func isRewrite(raw json.RawMessage) bool {
	s := bytes.TrimSpace(raw)
	return len(s) > 0 && string(s) != "null"
}

// decisionRank follows Claude's PreToolUse precedence: deny > defer > ask > allow.
// Unknown values rank as allow.
func decisionRank(decision string) int {
	switch decision {
	case "deny":
		return 4
	case "defer":
		return 3
	case "ask":
		return 2
	case "":
		return 0
	default:
		return 1
	}
}

// mergeClaudeReplies folds several Claude hook replies into one object.
//
//   - The decision is the highest-precedence one; ties keep the earlier reason.
//   - The first updatedInput wins. A winning deny drops any rewrite, with a note,
//     because a denied call must not run with a different input.
//   - additionalContext and systemMessage are joined with newlines, in order.
//   - Every other top-level key (continue, stopReason, suppressOutput, …) is
//     kept first-wins. A later reply that sets the same key to another value is
//     dropped with a note.
//
// Replies that do not parse as a JSON object are dropped with a note.
func mergeClaudeReplies(replies [][]byte) ([]byte, []string) {
	var notes []string
	var spec specificOutput
	hasSpec := false
	bestRank := 0
	var contexts, messages []string
	extra := map[string]json.RawMessage{}

	for i, raw := range replies {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
			notes = append(notes, fmt.Sprintf("reply %d is not a JSON object; dropped", i+1))
			continue
		}
		for key, value := range obj {
			switch key {
			case "hookSpecificOutput":
				if string(bytes.TrimSpace(value)) == "null" {
					continue
				}
				var hs specificOutput
				if err := json.Unmarshal(value, &hs); err != nil {
					notes = append(notes, fmt.Sprintf("reply %d hookSpecificOutput is not an object; dropped", i+1))
					continue
				}
				hasSpec = true
				mergeSpecific(&spec, &bestRank, hs, i+1, &notes, &contexts)
			case "systemMessage":
				var s string
				if err := json.Unmarshal(value, &s); err != nil {
					notes = append(notes, fmt.Sprintf("reply %d systemMessage is not a string; dropped", i+1))
					continue
				}
				if s = strings.TrimSpace(s); s != "" {
					messages = append(messages, s)
				}
			default:
				if first, ok := extra[key]; ok {
					if !bytes.Equal(bytes.TrimSpace(first), bytes.TrimSpace(value)) {
						notes = append(notes, fmt.Sprintf("reply %d %s dropped (first reply wins)", i+1, key))
					}
					continue
				}
				extra[key] = value
			}
		}
	}

	if spec.PermissionDecision == "deny" && isRewrite(spec.UpdatedInput) {
		notes = append(notes, "updatedInput dropped: a deny won the merge")
		spec.UpdatedInput = nil
	}
	spec.AdditionalContext = strings.Join(contexts, "\n")

	out := make(map[string]json.RawMessage, len(extra)+2)
	for key, value := range extra {
		out[key] = value
	}
	if len(messages) > 0 {
		b, err := json.Marshal(strings.Join(messages, "\n"))
		if err != nil {
			// Unreachable: a joined Go string always marshals.
			return []byte("{}\n"), append(notes, "merge failed: "+err.Error())
		}
		out["systemMessage"] = b
	}
	if hasSpec && !spec.empty() {
		b, err := json.Marshal(spec)
		if err != nil {
			// Unreachable: every field is a plain string or raw JSON we already parsed.
			return []byte("{}\n"), append(notes, "merge failed: "+err.Error())
		}
		out["hookSpecificOutput"] = b
	}
	if len(out) == 0 {
		return []byte("{}\n"), notes
	}
	merged, err := json.Marshal(out)
	if err != nil {
		return []byte("{}\n"), append(notes, "merge failed: "+err.Error())
	}
	return append(merged, '\n'), notes
}

// mergeSpecific folds one reply's hookSpecificOutput into spec.
func mergeSpecific(spec *specificOutput, bestRank *int, hs specificOutput, replyNo int, notes *[]string, contexts *[]string) {
	if spec.HookEventName == "" {
		spec.HookEventName = hs.HookEventName
	}
	if rank := decisionRank(hs.PermissionDecision); rank > *bestRank {
		*bestRank = rank
		spec.PermissionDecision = hs.PermissionDecision
		spec.PermissionDecisionReason = hs.PermissionDecisionReason
	}
	if isRewrite(hs.UpdatedInput) {
		if !isRewrite(spec.UpdatedInput) {
			spec.UpdatedInput = hs.UpdatedInput
		} else {
			*notes = append(*notes, fmt.Sprintf("second updatedInput from reply %d dropped (one rewrite per event)", replyNo))
		}
	}
	if s := strings.TrimSpace(hs.AdditionalContext); s != "" {
		*contexts = append(*contexts, s)
	}
}
