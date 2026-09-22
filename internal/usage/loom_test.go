package usage

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"codeberg.org/ubunatic/loom"
)

func TestUsageLoomWidget_RendersCorrectUI(t *testing.T) {
	now := time.Date(2026, 4, 18, 22, 36, 48, 0, time.UTC)

	summary := UsageSummary{
		Timestamp: now,
		Agents: []AgentUsage{
			{
				AgentID:       "claude",
				Name:          "Claude Code",
				Installed:     true,
				Authenticated: true,
				Weekly: &QuotaWindow{
					Name:        "Weekly",
					UsedPercent: 60,
					ResetAt:     pTime(now.Add(3*24*time.Hour + 20*time.Hour)),
				},
				Session: &QuotaWindow{
					Name:        "Session",
					UsedPercent: 59,
					ResetAt:     pTime(now.Add(1*time.Hour + 3*time.Minute)),
				},
			},
			{
				AgentID:       "agy",
				Name:          "Antigravity",
				Installed:     true,
				Authenticated: true,
				ModelGroups: []ModelGroup{
					{
						Name: "Gemini",
						Windows: []QuotaWindow{
							{
								Name:        "Weekly",
								UsedPercent: 97,
								ResetAt:     pTime(now.Add(12*time.Hour + 46*time.Minute)),
							},
							{
								Name:        "Session",
								UsedPercent: 0,
								ResetAt:     pTime(now.Add(4*time.Hour + 59*time.Minute)),
							},
						},
					},
					{
						Name: "Claude/GPT",
						Windows: []QuotaWindow{
							{
								Name:        "Weekly",
								UsedPercent: 92,
								ResetAt:     pTime(now.Add(4*24*time.Hour + 21*time.Hour)),
							},
							{
								Name:        "Session",
								UsedPercent: 0,
								ResetAt:     pTime(now.Add(4*time.Hour + 59*time.Minute)),
							},
						},
					},
				},
			},
			{
				AgentID:       "codex",
				Name:          "OpenAI Codex",
				Installed:     true,
				Authenticated: true,
				Weekly: &QuotaWindow{
					Name:        "Weekly",
					UsedPercent: 99,
					ResetAt:     pTime(now.Add(22*time.Hour + 5*time.Minute)),
				},
				Session: &QuotaWindow{
					Name:        "Session",
					UsedPercent: 4,
					ResetAt:     pTime(now.Add(4*time.Hour + 19*time.Minute)),
				},
			},
		},
	}

	widget := NewUsageLoomWidget(summary, WatchOptions{Compact: true}, "")
	widget.Now = now

	cols, rows := 100, 12
	rendered := loom.Render(widget, cols, rows)

	if len(rendered) == 0 {
		t.Fatalf("loom.Render produced no lines")
	}

	fullOutput := strings.Join(rendered, "\n")
	cleanOutput := stripANSI(fullOutput)

	if !strings.Contains(cleanOutput, "Agentic usage") {
		t.Errorf("expected output to contain 'Agentic usage', got:\n%s", cleanOutput)
	}

	if !strings.Contains(cleanOutput, "¹ All Usage") {
		t.Errorf("expected output to contain '¹ All Usage', got:\n%s", cleanOutput)
	}

	if !strings.Contains(cleanOutput, "⁷ Load") {
		t.Errorf("expected output to contain '⁷ Load', got:\n%s", cleanOutput)
	}

	for _, expected := range []string{"Claude Code", "Gemini", "Claude/GPT", "OpenAI Codex"} {
		if !strings.Contains(cleanOutput, expected) {
			t.Errorf("expected output to contain %q, got:\n%s", expected, cleanOutput)
		}
	}

	for _, expected := range []string{"cpu", "ram"} {
		if !strings.Contains(cleanOutput, expected) {
			t.Errorf("expected output to contain %q, got:\n%s", expected, cleanOutput)
		}
	}
}

func TestUsageLoomWidget_HandleKey(t *testing.T) {
	widget := NewUsageLoomWidget(UsageSummary{}, WatchOptions{Compact: true}, "")

	// Test 'q' key quits
	if !widget.HandleKey(loom.KeyEvent{Text: "q"}) {
		t.Errorf("expected 'q' key to signal quit")
	}

	// Test '?' toggles controls overlay
	widget.HandleKey(loom.KeyEvent{Text: "?"})
	if !widget.st.overlayOpen {
		t.Errorf("expected '?' key to toggle overlay open")
	}

	// Test dismissal with 'q'
	widget.HandleKey(loom.KeyEvent{Text: "q"})
	if widget.st.overlayOpen {
		t.Errorf("expected 'q' key to dismiss overlay")
	}
}

func TestUsageLoomWidget_SplashScreen(t *testing.T) {
	widget := NewUsageLoomWidget(UsageSummary{}, WatchOptions{Compact: true}, "")
	widget.splashActive = true
	widget.splashAnimate = true
	widget.splashStatusText = "fetching claude..."

	rendered := loom.Render(widget, 80, 10)
	output := stripANSI(strings.Join(rendered, "\n"))

	if !strings.Contains(output, "harnez usage") {
		t.Errorf("expected splash output to contain 'harnez usage', got:\n%s", output)
	}
	if !strings.Contains(output, "fetching claude...") {
		t.Errorf("expected splash output to contain status text, got:\n%s", output)
	}
}

func TestRunLoom_NonTerminalOutput(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()
	client := &http.Client{Timeout: 1 * time.Second}

	err := RunLoom(ctx, "", client, &buf, 60*time.Second, WatchOptions{})
	if err != nil {
		t.Fatalf("RunLoom failed: %v", err)
	}

	output := stripANSI(buf.String())
	if !strings.Contains(output, "Agentic usage") {
		t.Errorf("expected RunLoom output to contain 'Agentic usage', got:\n%s", output)
	}
	if !strings.Contains(output, "¹ All Usage") {
		t.Errorf("expected RunLoom output to contain '¹ All Usage', got:\n%s", output)
	}
	if !strings.Contains(output, "⁷ Load") {
		t.Errorf("expected RunLoom output to contain '⁷ Load', got:\n%s", output)
	}
}

func pTime(t time.Time) *time.Time {
	return &t
}
