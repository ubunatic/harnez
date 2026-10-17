package main

import "testing"

// skipShort marks a test as full tier (docs/TestTiers.md): it runs real
// subprocesses, binaries, process trees, or fixed-duration timers, so
// `go test -short` skips it.
func skipShort(t *testing.T, why string) {
	t.Helper()
	if testing.Short() {
		t.Skip("full test tier: " + why)
	}
}
