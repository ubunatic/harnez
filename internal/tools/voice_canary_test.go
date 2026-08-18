//go:build debug

package tools

import (
	"testing"
)

func TestValidateMelMultiple(t *testing.T) {
	tests := []struct {
		name    string
		val     float64
		wantErr bool
	}{
		{"valid 0.48", 0.48, false},
		{"valid 1.60", 1.60, false},
		{"valid 0.32", 0.32, false},
		{"valid 0.08", 0.08, false},
		{"invalid 0.50", 0.50, true},
		{"invalid 1.50", 1.50, true},
		{"invalid 0.45", 0.45, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMelMultiple("test", tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMelMultiple(test, %v) error = %v, wantErr %v", tt.val, err, tt.wantErr)
			}
		})
	}
}

func TestDefaultCanaryOptions(t *testing.T) {
	opts := DefaultCanaryOptions()
	if err := ValidateMelMultiple("chunk", opts.ChunkSecs); err != nil {
		t.Errorf("Default chunk is invalid: %v", err)
	}
	if err := ValidateMelMultiple("left", opts.LeftContextSecs); err != nil {
		t.Errorf("Default left context is invalid: %v", err)
	}
	if err := ValidateMelMultiple("right", opts.RightContextSecs); err != nil {
		t.Errorf("Default right context is invalid: %v", err)
	}
}
