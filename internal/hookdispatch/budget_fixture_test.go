package hookdispatch

import (
	"bytes"
	"testing"

	"ubunatic.com/harnez"
)

// budgetSpec returns the embedded spec/hooks.yaml with handler_timeout set to v,
// so budget tests exercise the full schema, including the agent tables.
func budgetSpec(t *testing.T, v string) []byte {
	t.Helper()
	raw, err := harnez.DefaultFS.ReadFile("spec/hooks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Replace(raw, []byte("handler_timeout: 40s"), []byte("handler_timeout: "+v), 1)
}
