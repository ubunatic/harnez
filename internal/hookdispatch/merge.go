package hookdispatch

import (
	"encoding/json"
	"fmt"
	"strings"
)

// claudeReply is the subset of a Claude hook reply that harnez handlers use.
// Keys a handler adds beyond these are dropped when two or more replies merge;
// a single reply never passes through this type.
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
		len(s.UpdatedInput) == 0 && s.AdditionalContext == ""
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

// mergeClaudeReplies folds several Claude hook replies into one. The decision
// is the highest-precedence one; ties keep the earlier handler's reason. The
// first updatedInput wins and later rewrites are dropped with a note.
// additionalContext and systemMessage are joined with newlines in order.
// Replies that do not parse as a JSON object are dropped with a note.
func mergeClaudeReplies(replies [][]byte) ([]byte, []string) {
	var notes []string
	var merged claudeReply
	var spec specificOutput
	hasSpec := false
	bestRank := 0
	var contexts, messages []string

	for i, raw := range replies {
		var r claudeReply
		if err := json.Unmarshal(raw, &r); err != nil {
			notes = append(notes, fmt.Sprintf("reply %d is not a JSON object; dropped", i+1))
			continue
		}
		if s := strings.TrimSpace(r.SystemMessage); s != "" {
			messages = append(messages, s)
		}
		hs := r.HookSpecificOutput
		if hs == nil {
			continue
		}
		hasSpec = true
		if spec.HookEventName == "" {
			spec.HookEventName = hs.HookEventName
		}
		if rank := decisionRank(hs.PermissionDecision); rank > bestRank {
			bestRank = rank
			spec.PermissionDecision = hs.PermissionDecision
			spec.PermissionDecisionReason = hs.PermissionDecisionReason
		}
		if len(hs.UpdatedInput) > 0 {
			if len(spec.UpdatedInput) == 0 {
				spec.UpdatedInput = hs.UpdatedInput
			} else {
				notes = append(notes, fmt.Sprintf("second updatedInput from reply %d dropped (one rewrite per event)", i+1))
			}
		}
		if s := strings.TrimSpace(hs.AdditionalContext); s != "" {
			contexts = append(contexts, s)
		}
	}

	if len(messages) > 0 {
		merged.SystemMessage = strings.Join(messages, "\n")
	}
	spec.AdditionalContext = strings.Join(contexts, "\n")
	if hasSpec && !spec.empty() {
		merged.HookSpecificOutput = &spec
	}
	out, err := json.Marshal(merged)
	if err != nil {
		// Unreachable: every field is a plain string or raw JSON we already parsed.
		return []byte("{}\n"), append(notes, "merge failed: "+err.Error())
	}
	if string(out) == "{}" {
		return []byte("{}\n"), notes
	}
	return append(out, '\n'), notes
}
