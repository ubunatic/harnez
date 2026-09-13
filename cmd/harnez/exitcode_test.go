package main

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestExitCodeFromRunError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil error is exit 0", nil, 0},
		{"exitCodeError uses its own code", &exitCodeError{Code: 42}, 42},
		{"exitCodeError code 0 stays 0", &exitCodeError{Code: 0}, 0},
		{"wrapped exitCodeError is unwrapped", errFmt("wrap", &exitCodeError{Code: 7}), 7},
		{"a plain error is exit 1", errors.New("boom"), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCodeFromRunError(tc.err); got != tc.want {
				t.Errorf("exitCodeFromRunError(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func errFmt(msg string, wrapped error) error {
	return &wrapError{msg: msg, err: wrapped}
}

type wrapError struct {
	msg string
	err error
}

func (e *wrapError) Error() string { return e.msg + ": " + e.err.Error() }
func (e *wrapError) Unwrap() error { return e.err }

func TestSilenceIfExitCode(t *testing.T) {
	t.Run("exitCodeError silences errors and usage", func(t *testing.T) {
		cmd := &cobra.Command{}
		err := silenceIfExitCode(cmd, &exitCodeError{Code: 1})
		if err == nil {
			t.Fatal("expected the error to be returned unchanged")
		}
		if !cmd.SilenceErrors || !cmd.SilenceUsage {
			t.Errorf("SilenceErrors=%v SilenceUsage=%v, want both true", cmd.SilenceErrors, cmd.SilenceUsage)
		}
	})

	t.Run("a genuine error leaves the command's defaults untouched", func(t *testing.T) {
		cmd := &cobra.Command{}
		err := silenceIfExitCode(cmd, errors.New("real failure"))
		if err == nil {
			t.Fatal("expected the error to be returned unchanged")
		}
		if cmd.SilenceErrors || cmd.SilenceUsage {
			t.Errorf("SilenceErrors=%v SilenceUsage=%v, want both false for a non-sentinel error", cmd.SilenceErrors, cmd.SilenceUsage)
		}
	})

	t.Run("nil error is a no-op", func(t *testing.T) {
		cmd := &cobra.Command{}
		if err := silenceIfExitCode(cmd, nil); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		if cmd.SilenceErrors || cmd.SilenceUsage {
			t.Error("nil error must not flip either silence flag")
		}
	})
}

func TestExitCodeErrorHasEmptyMessage(t *testing.T) {
	// Deliberate: Cobra's default error print becomes "Error: " (no text)
	// for this type if SilenceErrors were ever left false by mistake, which
	// is exactly the visible symptom a regression here would produce.
	if msg := (&exitCodeError{Code: 3}).Error(); msg != "" {
		t.Errorf("exitCodeError.Error() = %q, want empty string", msg)
	}
}
