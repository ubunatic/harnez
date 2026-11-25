package exec

import (
	"context"
	"strings"
	"testing"
)

func TestDiagnoseBwrapErrorClassifiesPrerequisites(t *testing.T) {
	cases := []struct {
		name     string
		output   string
		confined bool
		want     []string
	}{
		{
			name:   "mount slave host",
			output: "bwrap: Failed to make / slave: Operation not permitted",
			want:   []string{"Failed to make / slave", "host kernel/AppArmor"},
		},
		{
			name:   "uid map taia missing allow-bwrap",
			output: "bwrap: setting up uid map: Permission denied",
			want:   []string{"setting up uid map"},
		},
		{
			name:   "namespace creation",
			output: "bwrap: Creating new namespace failed: Operation not permitted",
			want:   []string{"namespace creation failed", "host kernel/AppArmor"},
		},
		{
			name:   "unrelated exit",
			output: "FAIL: TestSomething",
			want:   nil,
		},
		{
			name:   "unrelated mid-line mention",
			output: "FAIL: assertion mentioned bwrap: without startup prefix",
			want:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DiagnoseBwrapError(tc.output)
			if len(tc.want) == 0 && got != "" {
				t.Fatalf("DiagnoseBwrapError = %q, want empty", got)
			}
			for _, sub := range tc.want {
				if !strings.Contains(got, sub) {
					t.Fatalf("DiagnoseBwrapError = %q, missing %q", got, sub)
				}
			}
		})
	}
}

func TestProbeBwrapFailsClosedWithoutBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := ProbeBwrapSupport(context.Background()); err == nil || !strings.Contains(err.Error(), "bwrap") {
		t.Fatalf("ProbeBwrapSupport without bwrap = %v, want explicit error", err)
	}
}
