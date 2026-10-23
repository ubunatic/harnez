package main

import "testing"

func TestSharedFlagUsesReadOnlyCollectorSnapshot(t *testing.T) {
	seedUsageStore(t)
	root := newRootCmd()
	root.SetArgs([]string{"usage", "--shared", "--offline", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}
