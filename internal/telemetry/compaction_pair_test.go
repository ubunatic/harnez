package telemetry

import (
	"path/filepath"
	"testing"
)

func TestLatestCompactionPair(t *testing.T) {
	for _, tc := range []struct {
		name        string
		events      []CompactionEvent
		interrupted bool
		turn        string
	}{
		{"pair", []CompactionEvent{{EventType: "precompact", TurnID: "a"}, {EventType: "postcompact", TurnID: "a"}}, false, "a"},
		{"duplicates", []CompactionEvent{{EventType: "precompact", TurnID: "a"}, {EventType: "precompact", TurnID: "a"}, {EventType: "postcompact", TurnID: "a"}, {EventType: "postcompact", TurnID: "a"}}, false, "a"},
		{"interrupted", []CompactionEvent{{EventType: "precompact", TurnID: "a"}, {EventType: "postcompact", TurnID: "a"}, {EventType: "precompact", TurnID: "b"}, {EventType: "postcompact", TurnID: "a"}}, true, "b"},
		{"no turn", []CompactionEvent{{EventType: "precompact"}, {EventType: "postcompact"}, {EventType: "precompact"}}, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := Open(filepath.Join(t.TempDir(), "telemetry.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, e := range tc.events {
				e.SessionID = "s"
				e.Source = "stream_fallback"
				if _, err := db.InsertCompactionEvent(e); err != nil {
					t.Fatal(err)
				}
			}
			pair, err := db.LatestCompactionPair("s")
			if err != nil {
				t.Fatal(err)
			}
			if pair == nil || pair.Interrupted != tc.interrupted || pair.TurnID != tc.turn || pair.Source != "stream_fallback" {
				t.Fatalf("pair = %+v", pair)
			}
			other, err := db.LatestCompactionPair("other")
			if err != nil || other != nil {
				t.Fatalf("other = %+v, %v", other, err)
			}
		})
	}
}
