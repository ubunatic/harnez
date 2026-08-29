package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeCredentials models ~/.claude/.credentials.json
type ClaudeCredentials struct {
	ClaudeAiOauth struct {
		AccessToken           string   `json:"accessToken"`
		RefreshToken          string   `json:"refreshToken"`
		ExpiresAt             int64    `json:"expiresAt"`
		RefreshTokenExpiresAt int64    `json:"refreshTokenExpiresAt"`
		Scopes                []string `json:"scopes"`
		SubscriptionType      string   `json:"subscriptionType"`
		RateLimitTier         string   `json:"rateLimitTier"`
	} `json:"claudeAiOauth"`
}

// ClaudeStatsCache models ~/.claude/stats-cache.json
type ClaudeStatsCache struct {
	Version          int                                `json:"version"`
	LastComputedDate string                             `json:"lastComputedDate"`
	TotalSessions    int                                `json:"totalSessions"`
	TotalMessages    int                                `json:"totalMessages"`
	ModelUsage       map[string]ClaudeModelUsageMetrics `json:"modelUsage"`
}

// ClaudeModelUsageMetrics models stats within stats-cache.json
type ClaudeModelUsageMetrics struct {
	InputTokens              int64   `json:"inputTokens"`
	OutputTokens             int64   `json:"outputTokens"`
	CacheReadInputTokens     int64   `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int64   `json:"cacheCreationInputTokens"`
	WebSearchRequests        int64   `json:"webSearchRequests"`
	CostUSD                  float64 `json:"costUSD"`
}

// ClaudeOauthUsageResponse models GET https://api.anthropic.com/api/oauth/usage
type ClaudeOauthUsageResponse struct {
	FiveHour *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"five_hour"`
	SevenDay *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"seven_day"`
	Limits []struct {
		Kind     string  `json:"kind"`
		Group    string  `json:"group"`
		Percent  float64 `json:"percent"`
		Severity string  `json:"severity"`
		ResetsAt string  `json:"resets_at"`
		IsActive bool    `json:"is_active"`
	} `json:"limits"`
	ExtraUsage *struct {
		IsEnabled   bool    `json:"is_enabled"`
		UsedCredits float64 `json:"used_credits"`
		Currency    string  `json:"currency"`
	} `json:"extra_usage"`
}

// claudeQuotaPayload is Claude's live-fetch cache payload shape (see the
// shared liveFetchCache gate in livefetchcache.go, issue 033/087).
type claudeQuotaPayload struct {
	Session *QuotaWindow `json:"session,omitempty"`
	Weekly  *QuotaWindow `json:"weekly,omitempty"`
}

// CollectClaude inspects ~/.claude for credentials, cached stats, and queries live usage when online.
func CollectClaude(ctx context.Context, claudeDir string, client *http.Client) AgentUsage {
	usage := AgentUsage{
		AgentID:      "claude",
		Name:         "Claude Code",
		Details:      make(map[string]string),
		ExtraWindows: make(map[string]QuotaWindow),
	}

	if claudeDir == "" {
		home, _ := os.UserHomeDir()
		claudeDir = filepath.Join(home, ".claude")
	}

	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		usage.Installed = false
		return usage
	}
	usage.Installed = true

	// 1. Read settings.json for active configuration / model
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if data, err := os.ReadFile(settingsPath); err == nil {
		usage.Sources = append(usage.Sources, "~/.claude/settings.json")
		var s map[string]any
		if err := json.Unmarshal(data, &s); err == nil {
			if m, ok := s["model"].(string); ok && m != "" {
				usage.ActiveModel = m
			}
		}
	}

	// 2. Read stats-cache.json for local token tracking
	statsPath := filepath.Join(claudeDir, "stats-cache.json")
	if data, err := os.ReadFile(statsPath); err == nil {
		usage.Sources = append(usage.Sources, "~/.claude/stats-cache.json")
		var stats ClaudeStatsCache
		if err := json.Unmarshal(data, &stats); err == nil {
			tb := &TokenBreakdown{}
			modelToks := make(map[string]int64)
			for modelName, m := range stats.ModelUsage {
				modelTotal := m.InputTokens + m.OutputTokens + m.CacheReadInputTokens + m.CacheCreationInputTokens
				modelToks[modelName] = modelTotal
				tb.InputTokens += m.InputTokens
				tb.OutputTokens += m.OutputTokens
				tb.CacheReadTokens += m.CacheReadInputTokens
				tb.CacheWriteTokens += m.CacheCreationInputTokens
				tb.CostUSD += m.CostUSD
			}
			tb.TotalTokens = tb.InputTokens + tb.OutputTokens + tb.CacheReadTokens + tb.CacheWriteTokens
			usage.Tokens = tb
			if len(modelToks) > 0 {
				usage.ModelTokens = modelToks
			}
			if stats.TotalSessions > 0 {
				usage.Details["total_sessions"] = fmt.Sprintf("%d", stats.TotalSessions)
			}
			if stats.TotalMessages > 0 {
				usage.Details["total_messages"] = fmt.Sprintf("%d", stats.TotalMessages)
			}
		}
	}

	// 3. Read .credentials.json for OAuth & Plan info
	credsPath := filepath.Join(claudeDir, ".credentials.json")
	data, err := os.ReadFile(credsPath)
	if err != nil {
		usage.Authenticated = false
		return usage
	}
	usage.Sources = append(usage.Sources, "~/.claude/.credentials.json")

	var creds ClaudeCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		usage.Authenticated = false
		usage.Error = fmt.Sprintf("invalid credentials format: %v", err)
		return usage
	}

	oauth := creds.ClaudeAiOauth
	if oauth.AccessToken == "" {
		usage.Authenticated = false
		return usage
	}

	usage.Authenticated = true
	if oauth.SubscriptionType != "" {
		usage.PlanTier = strings.ToUpper(oauth.SubscriptionType[:1]) + oauth.SubscriptionType[1:]
	}
	if oauth.RateLimitTier != "" {
		usage.Details["rate_limit_tier"] = oauth.RateLimitTier
	}

	// 4. Query live quota endpoint if client is provided, but check the shared
	// on-disk cache first so a warm reading from a sibling `harnez` process
	// (or this process's own last tick) short-circuits the network call
	// entirely (issue 033).
	if client != nil {
		cachePath := liveFetchCachePath(claudeDir)
		defer lockLiveFetchInProcess(cachePath)()
		cache := readLiveFetchCache[claudeQuotaPayload](cachePath)

		if cache != nil && time.Since(cache.FetchedAt) < MinWatchInterval {
			usage.Session = cache.Payload.Session
			usage.Weekly = cache.Payload.Weekly
			usage.Sources = append(usage.Sources, "~/.claude/harnez-quota-cache.json")
			return usage
		}

		// Cache is stale or missing: fetch live. Take the advisory lock
		// first (bounded retry, never blocks indefinitely) so that only one
		// process at a time writes the refreshed reading to disk; if the
		// lock can't be acquired quickly, still perform the fetch and use
		// its result in-memory for this call, just skip persisting it (a
		// sibling process is presumably writing its own fresh reading right
		// now anyway).
		lockFile, locked := lockLiveFetchCache(cachePath)
		if locked {
			defer unlockLiveFetchCache(lockFile)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/api/oauth/usage", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+oauth.AccessToken)
			req.Header.Set("User-Agent", "claude-code/2.1.233")
			req.Header.Set("anthropic-client", "claude-code/2.1.233")
			req.Header.Set("anthropic-beta", "oauth-2025-04-20")
			req.Header.Set("Accept", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				usage.QuotaFetchError = err.Error()
			} else {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var oauthUsage ClaudeOauthUsageResponse
					if err := json.NewDecoder(resp.Body).Decode(&oauthUsage); err != nil {
						usage.QuotaFetchError = fmt.Sprintf("decode error: %v", err)
					} else {
						usage.Sources = append(usage.Sources, "api.anthropic.com/api/oauth/usage")
						now := time.Now()
						if oauthUsage.FiveHour != nil {
							qw := QuotaWindow{
								Name:             "Session (5-hour)",
								UsedPercent:      oauthUsage.FiveHour.Utilization,
								RemainingPercent: 100.0 - oauthUsage.FiveHour.Utilization,
							}
							if qw.RemainingPercent < 0 {
								qw.RemainingPercent = 0
							}
							if oauthUsage.FiveHour.ResetsAt != "" {
								if t, err := time.Parse(time.RFC3339Nano, oauthUsage.FiveHour.ResetsAt); err == nil {
									qw.ResetAt = &t
									if t.After(now) {
										qw.DurationLeft = t.Sub(now)
									}
								}
							}
							usage.Session = &qw
						}

						if oauthUsage.SevenDay != nil {
							qw := QuotaWindow{
								Name:             "Weekly (7-day)",
								UsedPercent:      oauthUsage.SevenDay.Utilization,
								RemainingPercent: 100.0 - oauthUsage.SevenDay.Utilization,
							}
							if qw.RemainingPercent < 0 {
								qw.RemainingPercent = 0
							}
							if oauthUsage.SevenDay.ResetsAt != "" {
								if t, err := time.Parse(time.RFC3339Nano, oauthUsage.SevenDay.ResetsAt); err == nil {
									qw.ResetAt = &t
									if t.After(now) {
										qw.DurationLeft = t.Sub(now)
									}
								}
							}
							usage.Weekly = &qw
						}

						if oauthUsage.ExtraUsage != nil && oauthUsage.ExtraUsage.IsEnabled {
							usage.Details["extra_usage"] = fmt.Sprintf("%.2f %s", oauthUsage.ExtraUsage.UsedCredits, oauthUsage.ExtraUsage.Currency)
						}
					}
				} else {
					usage.QuotaFetchError = fmt.Sprintf("HTTP %d", resp.StatusCode)
					usage.Details["live_quota_status"] = usage.QuotaFetchError
				}
			}
		} else {
			usage.QuotaFetchError = fmt.Sprintf("request build error: %v", err)
		}

		if usage.QuotaFetchError == "" {
			// Live fetch succeeded: persist it for sibling processes/next
			// tick, but only if we actually hold the lock.
			if locked {
				_ = writeLiveFetchCache(cachePath, liveFetchCache[claudeQuotaPayload]{
					FetchedAt: time.Now(),
					Payload:   claudeQuotaPayload{Session: usage.Session, Weekly: usage.Weekly},
				})
			}
		} else if cache != nil {
			// Live fetch failed: fall back to the disk cache regardless of
			// its age, labeled stale (issue 032's " (stale)" convention).
			// This subsumes issue 032's in-memory-only fallback for Claude:
			// that mechanism (applyStaleQuota in watch.go) still runs
			// afterward in RunWatch, but only fills windows still nil, so it
			// stays a harmless backstop for Claude (e.g. cache file missing)
			// and remains the active mechanism for AGY/Codex, which issue
			// 087 has since made active there too.
			if cache.Payload.Session != nil {
				usage.Session = staleQuotaWindow(cache.Payload.Session)
			}
			if cache.Payload.Weekly != nil {
				usage.Weekly = staleQuotaWindow(cache.Payload.Weekly)
			}
			usage.Sources = append(usage.Sources, "~/.claude/harnez-quota-cache.json (stale)")
		}
	}

	return usage
}
