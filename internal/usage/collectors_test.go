package usage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectClaude(t *testing.T) {
	// Setup temporary fake ~/.claude
	tempDir := t.TempDir()

	// 1. Write settings.json
	settingsJSON := `{"model": "claude-sonnet-5"}`
	if err := os.WriteFile(filepath.Join(tempDir, "settings.json"), []byte(settingsJSON), 0600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	// 2. Write stats-cache.json
	statsJSON := `{
		"version": 1,
		"totalSessions": 10,
		"totalMessages": 500,
		"modelUsage": {
			"claude-sonnet-5": {
				"inputTokens": 1000,
				"outputTokens": 2000,
				"cacheReadInputTokens": 3000,
				"cacheCreationInputTokens": 4000,
				"costUSD": 1.25
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(tempDir, "stats-cache.json"), []byte(statsJSON), 0600); err != nil {
		t.Fatalf("write stats-cache.json: %v", err)
	}

	// 3. Write .credentials.json
	credsJSON := `{
		"claudeAiOauth": {
			"accessToken": "mock-token",
			"subscriptionType": "max",
			"rateLimitTier": "default_max"
		}
	}`
	if err := os.WriteFile(filepath.Join(tempDir, ".credentials.json"), []byte(credsJSON), 0600); err != nil {
		t.Fatalf("write credentials.json: %v", err)
	}

	// 4. Create mock HTTP server for live quota
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mock-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"five_hour": {
				"utilization": 80.0,
				"resets_at": "2026-08-17T23:00:00Z"
			},
			"seven_day": {
				"utilization": 25.0,
				"resets_at": "2026-08-22T19:00:00Z"
			}
		}`))
	}))
	defer mockServer.Close()

	// Test collection
	ctx := context.Background()
	usage := CollectClaude(ctx, tempDir, nil)

	if !usage.Installed {
		t.Errorf("expected Installed = true")
	}
	if !usage.Authenticated {
		t.Errorf("expected Authenticated = true")
	}
	if usage.PlanTier != "Max" {
		t.Errorf("expected PlanTier = Max, got %q", usage.PlanTier)
	}
	if usage.ActiveModel != "claude-sonnet-5" {
		t.Errorf("expected ActiveModel = claude-sonnet-5, got %q", usage.ActiveModel)
	}
	if usage.Tokens == nil {
		t.Fatalf("expected Tokens != nil")
	}
	if usage.Tokens.TotalTokens != 10000 {
		t.Errorf("expected TotalTokens = 10000, got %d", usage.Tokens.TotalTokens)
	}
}

func TestCollectCodex(t *testing.T) {
	tempDir := t.TempDir()

	configTOML := `model = "gpt-5.6-sol"
model_reasoning_effort = "low"
`
	if err := os.WriteFile(filepath.Join(tempDir, "config.toml"), []byte(configTOML), 0600); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	rawPayload := `{"https://api.openai.com/profile":{"email":"test@example.com"},"https://api.openai.com/auth":{"chatgpt_plan_type":"plus"},"exp":2500000000}`
	b64Payload := "eyJodHRwczovL2FwaS5vcGVuYWkuY29tL3Byb2ZpbGUiOnsiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIn0sImh0dHBzOi8vYXBpLm9wZW5haS5jb20vYXV0aCI6eyJjaGF0Z3B0X3BsYW5fdHlwZSI6InBsdXMifSwiZXhwIjoyNTAwMDAwMDAwfQ"
	jwtToken := "eyJhbGciOiJIUzI1NiJ9." + b64Payload + ".signature"
	_ = rawPayload

	authJSON := `{
		"auth_mode": "chatgpt",
		"tokens": {
			"id_token": "` + jwtToken + `",
			"access_token": "mock-access-token",
			"account_id": "mock-account-id"
		}
	}`
	if err := os.WriteFile(filepath.Join(tempDir, "auth.json"), []byte(authJSON), 0600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}

	ctx := context.Background()
	usage := CollectCodex(ctx, tempDir, nil)

	if !usage.Installed {
		t.Errorf("expected Installed = true")
	}
	if !usage.Authenticated {
		t.Errorf("expected Authenticated = true")
	}
	if usage.PlanTier != "Plus" {
		t.Errorf("expected PlanTier = Plus, got %q", usage.PlanTier)
	}
	if usage.ActiveModel != "gpt-5.6-sol" {
		t.Errorf("expected ActiveModel = gpt-5.6-sol, got %q", usage.ActiveModel)
	}
	if usage.Account != "t***t@example.com" {
		t.Errorf("expected Account = t***t@example.com, got %q", usage.Account)
	}
}

func TestCollectAGY(t *testing.T) {
	tempDir := t.TempDir()

	settingsJSON := `{"model": "Gemini 3.7 Flash (Low)"}`
	if err := os.WriteFile(filepath.Join(tempDir, "settings.json"), []byte(settingsJSON), 0600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	tokenJSON := `{"token": "test-token", "auth_method": "consumer"}`
	if err := os.WriteFile(filepath.Join(tempDir, "antigravity-oauth-token"), []byte(tokenJSON), 0600); err != nil {
		t.Fatalf("write antigravity-oauth-token: %v", err)
	}

	logDir := filepath.Join(tempDir, "log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir log: %v", err)
	}
	logContent := "2026-08-11 server_oauth.go: applyAuthResult: email=dev@example.org, authMethod=consumer\n"
	if err := os.WriteFile(filepath.Join(logDir, "cli-123.log"), []byte(logContent), 0600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	ctx := context.Background()
	usage := CollectAGY(ctx, tempDir, nil)

	if !usage.Installed {
		t.Errorf("expected Installed = true")
	}
	if !usage.Authenticated {
		t.Errorf("expected Authenticated = true")
	}
	if usage.PlanTier != "Consumer" {
		t.Errorf("expected PlanTier = Consumer, got %q", usage.PlanTier)
	}
	if usage.ActiveModel != "Gemini 3.7 Flash (Low)" {
		t.Errorf("expected ActiveModel = Gemini 3.7 Flash (Low), got %q", usage.ActiveModel)
	}
	if usage.Account != "d***v@example.org" {
		t.Errorf("expected Account = d***v@example.org, got %q", usage.Account)
	}
}

func TestCollectAGY_NoTokenFile_LogAuth(t *testing.T) {
	tempDir := t.TempDir()

	settingsJSON := `{"model": "Gemini 3.7 Flash (Low)"}`
	if err := os.WriteFile(filepath.Join(tempDir, "settings.json"), []byte(settingsJSON), 0600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	logDir := filepath.Join(tempDir, "log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir log: %v", err)
	}
	logContent := "2026-08-20 server_oauth.go: applyAuthResult: email=user@domain.com, authMethod=consumer, quotaProject=\n"
	if err := os.WriteFile(filepath.Join(logDir, "cli-456.log"), []byte(logContent), 0600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	ctx := context.Background()
	usage := CollectAGY(ctx, tempDir, nil)

	if !usage.Installed {
		t.Errorf("expected Installed = true")
	}
	if !usage.Authenticated {
		t.Errorf("expected Authenticated = true")
	}
	if usage.PlanTier != "Consumer" {
		t.Errorf("expected PlanTier = Consumer, got %q", usage.PlanTier)
	}
	if usage.ActiveModel != "Gemini 3.7 Flash (Low)" {
		t.Errorf("expected ActiveModel = Gemini 3.7 Flash (Low), got %q", usage.ActiveModel)
	}
	if usage.Account != "u***r@domain.com" {
		t.Errorf("expected Account = u***r@domain.com, got %q", usage.Account)
	}
}

func TestCollectAGY_SettingsOnlyIsNotAuthenticated(t *testing.T) {
	tempDir := t.TempDir()

	settingsJSON := `{"model": "Gemini 3.7 Flash (Low)"}`
	if err := os.WriteFile(filepath.Join(tempDir, "settings.json"), []byte(settingsJSON), 0600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	usage := CollectAGY(context.Background(), tempDir, nil)

	if usage.Authenticated {
		t.Errorf("expected settings-only installation to remain unauthenticated")
	}
	if usage.PlanTier != "" {
		t.Errorf("expected empty PlanTier, got %q", usage.PlanTier)
	}
	if usage.ActiveModel != "Gemini 3.7 Flash (Low)" {
		t.Errorf("expected ActiveModel = Gemini 3.7 Flash (Low), got %q", usage.ActiveModel)
	}
}
