package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTelemetrySpoolCapsAndDrainsRecords(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	first := []byte(`{"kind":"tool_call","payload":{"tool_name":"one"}}`)
	second := []byte(`{"kind":"tool_call","payload":{"tool_name":"two"}}`)
	if err := AppendTelemetrySpool(dir, first, int64(len(first)+len(second)-1)); err != nil {
		t.Fatal(err)
	}
	if err := AppendTelemetrySpool(dir, second, int64(len(first)+len(second)-1)); err == nil {
		t.Fatal("AppendTelemetrySpool exceeded configured cap")
	}
	var drained string
	if err := DrainTelemetrySpool(context.Background(), dir, func(_ context.Context, data []byte) error { drained = string(data); return nil }); err != nil {
		t.Fatal(err)
	}
	if drained != string(first) {
		t.Fatalf("drained %q, want %q", drained, first)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			t.Fatalf("spool record remains after successful drain: %s", entry.Name())
		}
	}
}

func TestTelemetrySpoolRetainsRecordWhenConsumerFails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	data := []byte(`{"kind":"codex","payload":{}}`)
	if err := AppendTelemetrySpool(dir, data, 4096); err != nil {
		t.Fatal(err)
	}
	wantErr := context.Canceled
	if err := DrainTelemetrySpool(context.Background(), dir, func(context.Context, []byte) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("DrainTelemetrySpool error = %v, want %v", err, wantErr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	remaining := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			remaining++
		}
	}
	if remaining != 1 {
		t.Fatalf("remaining records = %d, want 1", remaining)
	}
}

func TestTelemetrySpoolKeepsOrderAndContinuesPastFailedRecords(t *testing.T) {
	dir := t.TempDir()
	for _, data := range []string{"pre", "post", "later"} {
		if err := AppendTelemetrySpool(dir, []byte(data), 4096); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	err := DrainTelemetrySpool(context.Background(), dir, func(_ context.Context, data []byte) error {
		got = append(got, string(data))
		if string(data) == "post" {
			return context.Canceled
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) || len(got) != 3 || got[0] != "pre" || got[1] != "post" || got[2] != "later" {
		t.Fatalf("drain = %v, %v", got, err)
	}
}
