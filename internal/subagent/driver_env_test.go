package subagent

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDriverExportsSessionNameAndPID(t *testing.T) {
	os.Setenv("HARNEZ_SESSION_ID", "test-session-123")
	os.Setenv("HARNEZ_AGENT_LAUNCHER_PID", "999")
	defer os.Unsetenv("HARNEZ_SESSION_ID")
	defer os.Unsetenv("HARNEZ_AGENT_LAUNCHER_PID")

	ctx := context.Background()
	b, err := runProviderCLI(ctx, "env", ".", nil, nil, nil)
	if err != nil {
		t.Fatalf("runProviderCLI failed: %v", err)
	}
	out := string(b)
	if !strings.Contains(out, "HARNEZ_AGENT_SESSION=test-session-123") {
		t.Errorf("missing HARNEZ_AGENT_SESSION in env output:\n%s", out)
	}
	if !strings.Contains(out, "HARNEZ_AGENT_PID=999") {
		t.Errorf("missing HARNEZ_AGENT_PID in env output:\n%s", out)
	}
}
