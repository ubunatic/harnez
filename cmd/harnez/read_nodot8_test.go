//go:build !dot8

package main

import "testing"

func TestReadCmd_Dot8FlagsExcludedByDefault(t *testing.T) {
	cmd := newReadCmd()
	for _, name := range []string{"dot8", "dot8-colors", "dot8-pitch"} {
		f := cmd.Flags().Lookup(name)
		if f != nil {
			t.Errorf("flag %q should not be registered in default build", name)
		}
	}
}
