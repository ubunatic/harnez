package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	RunnerUsageActionLatestSummary  = "latest_summary"
	RunnerUsageActionProviderQuota  = "provider_quota"
	RunnerUsageActionHistoryEntries = "history_entries"
	RunnerUsageActionQuotaHistory   = "quota_history"
)

// RunnerUsageQuery requests a read-only canonical usage projection from the
// host collector without permitting caller-chosen database or directory paths.
type RunnerUsageQuery struct {
	Action   string   `json:"action"`
	Provider string   `json:"provider,omitempty"`
	Models   []string `json:"models,omitempty"`
}

// ValidateRunnerUsageQuery verifies that q is an allowlisted read-only query
// within configured spec bounds.
func ValidateRunnerUsageQuery(cfg Config, q RunnerUsageQuery) error {
	maxStr := cfg.RunnerChannel.UsageQueryMaxStringLen
	maxModels := cfg.RunnerChannel.UsageQueryMaxModels
	switch q.Action {
	case RunnerUsageActionLatestSummary, RunnerUsageActionHistoryEntries, RunnerUsageActionQuotaHistory:
		if q.Provider != "" || len(q.Models) > 0 {
			return fmt.Errorf("unexpected provider or models on %s query", q.Action)
		}
		return nil
	case RunnerUsageActionProviderQuota:
		p := strings.TrimSpace(q.Provider)
		if p == "" || (maxStr > 0 && len(p) > maxStr) || len(q.Models) == 0 {
			return fmt.Errorf("provider_quota query requires bounded provider and models")
		}
		if maxModels > 0 && len(q.Models) > maxModels {
			return fmt.Errorf("provider_quota query exceeds %d models", maxModels)
		}
		for _, m := range q.Models {
			if strings.TrimSpace(m) == "" || (maxStr > 0 && len(m) > maxStr) {
				return fmt.Errorf("provider_quota model name invalid or exceeds limit")
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported usage query action %q", ErrRunnerForbidden, q.Action)
	}
}

// QueryRunnerUsage executes an allowlisted read-only usage query over the
// inherited runner broker FD and unmarshals its JSON result into out.
func QueryRunnerUsage(ctx context.Context, cfg Config, q RunnerUsageQuery, out any) error {
	if err := ValidateRunnerUsageQuery(cfg, q); err != nil {
		return err
	}
	rawPayload, err := json.Marshal(q)
	if err != nil {
		return err
	}
	resp, err := SendRunnerRequest(ctx, cfg, RunnerRequest{
		Op:      RunnerOpUsageRead,
		Payload: rawPayload,
	})
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		msg := strings.TrimSpace(resp.Error)
		if msg == "" {
			msg = http.StatusText(resp.Status)
		}
		if resp.Status == http.StatusForbidden {
			return fmt.Errorf("%w: %s", ErrRunnerForbidden, msg)
		}
		if resp.Status == http.StatusServiceUnavailable {
			return fmt.Errorf("%w: %s", ErrRunnerUnavailable, msg)
		}
		return &HTTPStatusError{StatusCode: resp.Status, Status: http.StatusText(resp.Status), Body: msg}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		return fmt.Errorf("decode usage query result: %w", err)
	}
	return nil
}

func hostSocketUsageQuery(hostSocket string, cfg Config) func(context.Context, RunnerUsageQuery) ([]byte, error) {
	return func(callCtx context.Context, q RunnerUsageQuery) ([]byte, error) {
		body, err := json.Marshal(q)
		if err != nil {
			return nil, err
		}
		result, err := PostTelemetryCommand(callCtx, hostSocket, cfg.UsageQueryPath, body, cfg.HTTPClientTimeout)
		if err != nil {
			var statusErr *HTTPStatusError
			if errors.As(err, &statusErr) {
				return nil, statusErr
			}
			return nil, fmt.Errorf("%w: %v", ErrRunnerUnavailable, err)
		}
		return result, nil
	}
}

func handleRunnerUsageRead(ctx context.Context, cfg Config, payload json.RawMessage, query func(context.Context, RunnerUsageQuery) ([]byte, error)) RunnerResponse {
	if query == nil {
		return RunnerResponse{Status: http.StatusServiceUnavailable, Error: ErrRunnerUnavailable.Error()}
	}
	var q RunnerUsageQuery
	if err := decodeStrictJSON(payload, &q); err != nil {
		return RunnerResponse{Status: http.StatusBadRequest, Error: "invalid usage query"}
	}
	if err := ValidateRunnerUsageQuery(cfg, q); err != nil {
		return statusForRunnerErr(err)
	}
	raw, err := query(ctx, q)
	if err != nil {
		var statusErr *HTTPStatusError
		if errors.As(err, &statusErr) {
			return RunnerResponse{Status: statusErr.StatusCode, Error: strings.TrimSpace(statusErr.Body)}
		}
		if errors.Is(err, ErrRunnerForbidden) {
			return RunnerResponse{Status: http.StatusForbidden, Error: err.Error()}
		}
		return RunnerResponse{Status: http.StatusServiceUnavailable, Error: err.Error()}
	}
	maxPayload := cfg.RunnerChannel.MaxResultPayloadBytes()
	if maxPayload <= 0 || int64(len(raw)) > maxPayload {
		return RunnerResponse{Status: http.StatusRequestEntityTooLarge, Error: ErrRunnerPayloadTooLarge.Error()}
	}
	return RunnerResponse{Status: http.StatusOK, Result: json.RawMessage(raw)}
}
