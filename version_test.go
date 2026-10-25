package harnez

import "testing"

func TestBuildVersion(t *testing.T) {
	oldVersion, oldCount := Version, CommitCount
	t.Cleanup(func() { Version, CommitCount = oldVersion, oldCount })
	Version = "1.2.3"
	CommitCount = "42"
	if got := BuildVersion(); got != "1.2.3+42" {
		t.Fatalf("BuildVersion() = %q, want %q", got, "1.2.3+42")
	}
	CommitCount = ""
	if got := BuildVersion(); got != "1.2.3" {
		t.Fatalf("BuildVersion() without count = %q, want %q", got, "1.2.3")
	}
}
