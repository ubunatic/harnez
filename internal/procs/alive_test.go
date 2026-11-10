package procs

import (
	"os"
	"testing"
)

func TestIsAlive_Self(t *testing.T) {
	if !IsAlive(os.Getpid()) {
		t.Fatal("self not alive")
	}
}
