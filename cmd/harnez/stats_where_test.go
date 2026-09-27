package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

func TestRunStatsWhereListsStoresAndFlagsEmptyFiles(t *testing.T) {
	home, data, cache, state := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("XDG_STATE_HOME", state)

	dbPath := filepath.Join(data, "harnez", "telemetry.sqlite")
	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC)
	call := sampleCall("s1", "Read", 5, 0)
	call.CreatedAt = stamp
	if err := db.Insert(call); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	decoy := filepath.Join(data, "harnez", "telemetry.db")
	if err := os.WriteFile(decoy, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	cmd := newStatsCmd()
	cmd.SetArgs([]string{"--where", "--json"})
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var stores []telemetryStore
	if err := json.Unmarshal(output.Bytes(), &stores); err != nil {
		t.Fatalf("decode --where JSON: %v\n%s", err, output.String())
	}
	var foundTelemetry, foundDecoy bool
	for _, store := range stores {
		switch store.Path {
		case dbPath:
			foundTelemetry = true
			if store.Owner != "telemetry" || !store.Newest.Equal(stamp) || store.SizeBytes == 0 {
				t.Errorf("telemetry store = %+v", store)
			}
		case decoy:
			foundDecoy = true
			if store.Status != "EMPTY" || !strings.Contains(store.Answers, "Obsolete") {
				t.Errorf("decoy store = %+v", store)
			}
		}
	}
	if !foundTelemetry || !foundDecoy {
		t.Fatalf("stores omitted telemetry=%v decoy=%v: %+v", foundTelemetry, foundDecoy, stores)
	}
}
