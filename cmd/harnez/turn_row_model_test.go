package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usagestore"
)

// The token rows name the model the main-chain call reports, not the session's
// alias (ticket 1301 M4). A bare alias would get the wrong rates once a newer
// model ships.
func TestTokenRowsNameTheReportedModelNotTheAlias(t *testing.T) {
	cases := []struct {
		name, resultModel, want string
	}{
		{name: "reported model", resultModel: "claude-opus-5-5", want: "claude-opus-5-5"},
		{name: "no reported model keeps the session model", resultModel: "", want: "opus"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
			sess := &subagent.Session{
				ID: "s-model", Provider: "claude", Model: "opus", Turn: 1, TokenTotalsKnown: true,
				InputTokensTotal: 2, CachedTokensTotal: 0, OutputTokensTotal: 4,
				TurnRecords: []subagent.TurnRecord{{Turn: 1}},
			}
			result := &subagent.TurnResult{InputTokens: 2, OutputTokens: 4, Model: tc.resultModel}
			persistAgentTurnTokens(dbPath, sess, result, time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC))

			store, err := usagestore.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			rows, err := store.TurnTokenUsages(context.Background(), "s-model")
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 {
				t.Fatalf("rows = %+v; want one delta and one cumulative row", rows)
			}
			for _, row := range rows {
				if row.Model != tc.want {
					t.Fatalf("%s row model = %q; want %q", row.CounterKind, row.Model, tc.want)
				}
			}
		})
	}
}
