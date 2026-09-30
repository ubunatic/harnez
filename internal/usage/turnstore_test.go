package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

func TestImportTurnQuotaJSONLIsIdempotentAndPreservesParity(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "telemetry.sqlite")
	path := filepath.Join(dir, "quota-readings.jsonl")
	beforeAt := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	afterAt := beforeAt.Add(time.Minute)
	reset := beforeAt.Add(5 * time.Hour)
	events := []struct {
		SessionID string           `json:"session_id"`
		Turn      int              `json:"turn"`
		Boundary  string           `json:"boundary"`
		Provider  string           `json:"provider"`
		Reading   TurnQuotaReading `json:"reading"`
		Tokens    *struct {
			NewInputTokens    int `json:"new_input_tokens"`
			CachedInputTokens int `json:"cached_input_tokens"`
			OutputTokens      int `json:"output_tokens"`
		} `json:"tokens,omitempty"`
	}{
		{SessionID: "s1", Turn: 1, Boundary: "before", Provider: "codex", Reading: TurnQuotaReading{CapturedAt: beforeAt, HasCache: true, Windows: []QuotaHistoryEntry{{Timestamp: beforeAt, Agent: "codex", Window: "5-hour", UsedPercent: 20, ResetAt: &reset}}}},
		{SessionID: "s1", Turn: 1, Boundary: "after", Provider: "codex", Reading: TurnQuotaReading{CapturedAt: afterAt, HasCache: true, Windows: []QuotaHistoryEntry{{Timestamp: afterAt, Agent: "codex", Window: "5-hour", UsedPercent: 27, ResetAt: &reset}}}, Tokens: &struct {
			NewInputTokens    int `json:"new_input_tokens"`
			CachedInputTokens int `json:"cached_input_tokens"`
			OutputTokens      int `json:"output_tokens"`
		}{NewInputTokens: 90, CachedInputTokens: 10, OutputTokens: 15}},
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := json.NewEncoder(f).Encode(event); err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	first, err := ImportTurnQuotaJSONL(ctx, dbPath, path)
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceEvents != 2 || first.ImportedRows != 2 || first.InvalidRows != 0 || first.AlreadyDone {
		t.Fatalf("first import = %+v", first)
	}
	second, err := ImportTurnQuotaJSONL(ctx, dbPath, path)
	if err != nil {
		t.Fatal(err)
	}
	if second.SourceEvents != 2 || second.ImportedRows != 2 || second.InvalidRows != 0 || !second.AlreadyDone {
		t.Fatalf("second import = %+v", second)
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	deltas, err := store.TurnQuotaDeltas(ctx, "s1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 || deltas[0].WindowKey != "five_hour" || deltas[0].Delta != .07 {
		t.Fatalf("imported quota deltas = %+v", deltas)
	}
	tokens, err := store.TurnTokenUsages(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 || tokens[0].CounterKind != "cumulative" || tokens[1].CounterKind != "delta" || tokens[1].InputTokens == nil || *tokens[1].InputTokens != 90 {
		t.Fatalf("imported token rows = %+v", tokens)
	}
}
