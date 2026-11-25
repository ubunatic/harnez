package claude

import "testing"

func TestManagedTextForSwitchesTestTarget(t *testing.T) {
	const text = "run `make test-q1` before pushing"
	if got := managedTextFor(text, true); got != text {
		t.Errorf("quota on: managedTextFor = %q, want unchanged", got)
	}
	want := "run `make test` before pushing"
	if got := managedTextFor(text, false); got != want {
		t.Errorf("quota off: managedTextFor = %q, want %q", got, want)
	}
}
