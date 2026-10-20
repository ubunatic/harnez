package server

import (
	"encoding/json"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/readcard"
	"ubunatic.com/harnez/internal/telemetry"
)

func InsertToolCallAt(dbPath string, call telemetry.ToolCall) error {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Insert(call)
}

func InsertToolCallPayload(dbPath string, raw []byte) error {
	var call telemetry.ToolCall
	if err := json.Unmarshal(raw, &call); err != nil {
		return err
	}
	return InsertToolCallAt(dbPath, call)
}

func ProcessAgyPostTool(dbPath string, raw []byte) error {
	var payload struct {
		ConversationID string `json:"conversationId"`
		Output         string `json:"output"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	output := payload.Output
	actualTokens := int64(readcard.ComputeTextTokens(output).TextTokens)
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	var duration int64
	var savingsTokens, savingsBytes *int64
	calls, err := db.Query(telemetry.Filter{SessionID: payload.ConversationID})
	if err == nil && len(calls) > 0 {
		if !calls[0].CreatedAt.IsZero() {
			d := time.Since(calls[0].CreatedAt).Milliseconds()
			if d > 0 {
				duration = d
			}
		}
		if nativeReadTool(calls[0].ToolName) && output != "" {
			estimate := readcard.EstimateSavings(output, readcard.ParseProvider(calls[0].AgentID))
			savingsTokens = &estimate.SavingsTokens
			savingsBytes = &estimate.SavingsBytes
		}
	}
	return db.UpdateLatestToolCallMetrics(payload.ConversationID, int64(len([]byte(output))), duration, &actualTokens, savingsTokens, savingsBytes)
}

func nativeReadTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "view_file", "readmultiplefiles", "read_multiple_files", "cat":
		return true
	default:
		return false
	}
}
