package usage

import (
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestRemoteLoadCacheRoundTripByHost(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	want := &LoadSnapshot{CPU: CPULoad{NumCPU: 12, CPUPercent: 31, CPUPercentOk: true}}
	if err := WriteCachedRemoteLoad("host-a", want); err != nil {
		t.Fatal(err)
	}
	if got := ReadCachedRemoteLoad("host-a"); !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCachedRemoteLoad(host-a) = %#v, want %#v", got, want)
	}
	if got := ReadCachedRemoteLoad("host-b"); got != nil {
		t.Fatalf("ReadCachedRemoteLoad(host-b) = %#v, want nil", got)
	}
	entries, err := os.ReadDir(StateDir(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("cache files = %d, want 1", len(entries))
	}
}

// Issue 879 M2b: the reader returns the write time beside the snapshot.
func TestReadCachedRemoteLoadEntryReturnsWriteTime(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	before := time.Now()
	if err := WriteCachedRemoteLoad("host-a", &LoadSnapshot{CPU: CPULoad{NumCPU: 4}}); err != nil {
		t.Fatal(err)
	}
	after := time.Now()
	got := ReadCachedRemoteLoadEntry("host-a")
	if got.Snapshot == nil || got.Snapshot.CPU.NumCPU != 4 {
		t.Fatalf("ReadCachedRemoteLoadEntry snapshot = %#v, want NumCPU 4", got.Snapshot)
	}
	if got.UpdatedAt.Before(before) || got.UpdatedAt.After(after) {
		t.Fatalf("UpdatedAt = %s, want between %s and %s", got.UpdatedAt, before, after)
	}
}

// A legacy cache file has no updated_at: the snapshot still loads, with no time.
func TestReadCachedRemoteLoadEntryLegacyHasNoWriteTime(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := WriteCachedRemoteLoad("host-a", &LoadSnapshot{CPU: CPULoad{NumCPU: 4}}); err != nil {
		t.Fatal(err)
	}
	legacy := `{"host":"host-a","snapshot":{"cpu":{"NumCPU":4}}}`
	if err := os.WriteFile(remoteLoadCachePath("host-a"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	got := ReadCachedRemoteLoadEntry("host-a")
	if got.Snapshot == nil || got.Snapshot.CPU.NumCPU != 4 {
		t.Fatalf("legacy snapshot = %#v, want NumCPU 4", got.Snapshot)
	}
	if !got.UpdatedAt.IsZero() {
		t.Fatalf("legacy UpdatedAt = %s, want zero", got.UpdatedAt)
	}
}

func TestReadCachedRemoteLoadEntryMalformedIsEmpty(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := WriteCachedRemoteLoad("host-a", &LoadSnapshot{CPU: CPULoad{NumCPU: 4}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(remoteLoadCachePath("host-a"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ReadCachedRemoteLoadEntry("host-a"); got.Snapshot != nil || !got.UpdatedAt.IsZero() {
		t.Fatalf("malformed cache = %#v, want empty", got)
	}
}

// Collector and -w writers race on one host's file: every read must parse, and
// no temp file may be left behind (issue 879 M2b, atomic rename).
func TestWriteCachedRemoteLoadConcurrentWritersLeaveReadableEntry(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var writers sync.WaitGroup
	for writer := range 8 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range 20 {
				_ = WriteCachedRemoteLoad("host-a", &LoadSnapshot{CPU: CPULoad{NumCPU: writer + 1}})
			}
		}()
	}
	writers.Wait()
	got := ReadCachedRemoteLoadEntry("host-a")
	if got.Snapshot == nil || got.UpdatedAt.IsZero() {
		t.Fatalf("concurrent writes left an unreadable entry: %#v", got)
	}
	entries, err := os.ReadDir(StateDir(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("cache files = %d, want 1 (no temp files left)", len(entries))
	}
}
