package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

type telemetryCollectorService struct {
	config       server.Config
	path         string
	registryDir  string
	interval     time.Duration
	warnMu       sync.Mutex
	warnedRecord map[string]time.Time
}

func (s *telemetryCollectorService) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+s.config.TelemetryPath, s.handleRecord)
	mux.HandleFunc("POST "+s.config.UsageQueryPath, s.handleUsageQuery)
	mux.HandleFunc("POST "+s.config.RegistryPath, s.handleRegistry)
}

func (s *telemetryCollectorService) handleRecord(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.config.UpdateMaxBodyBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid telemetry record", http.StatusBadRequest)
		return
	}
	var record server.TelemetryRecord
	if err := json.Unmarshal(data, &record); err != nil || record.Kind == "" || len(record.Payload) == 0 {
		http.Error(w, "invalid telemetry record", http.StatusBadRequest)
		return
	}
	body, err := processTelemetryRecordResult(r.Context(), record, s.path)
	if err != nil {
		http.Error(w, "telemetry write failed", http.StatusServiceUnavailable)
		return
	}
	if body != nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *telemetryCollectorService) handleUsageQuery(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.config.UpdateMaxBodyBytes)
	var q server.RunnerUsageQuery
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&q); err != nil {
		http.Error(w, "invalid usage query", http.StatusBadRequest)
		return
	}
	if err := server.ValidateRunnerUsageQuery(s.config, q); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	maxPayload := s.config.RunnerChannel.MaxResultPayloadBytes()
	switch q.Action {
	case server.RunnerUsageActionLatestSummary:
		summary, err := usage.ReadStoredSummaryDirect(r.Context(), s.path)
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, usage.ErrNoCollectorSnapshots) || errors.Is(err, os.ErrNotExist) {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSONResponse(w, summary)
	case server.RunnerUsageActionProviderQuota:
		avail := usage.ProviderQuotaAvailabilitiesFromStoreReadOnly(r.Context(), s.path, q.Provider, q.Models, time.Now())
		writeJSONResponse(w, avail)
	case server.RunnerUsageActionHistoryEntries:
		entries, err := usage.ReadUsageHistoryFromStore(r.Context(), s.path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		writeCappedTailJSON(w, entries, maxPayload)
	case server.RunnerUsageActionQuotaHistory:
		entries, err := usage.ReadQuotaHistoryFromStore(r.Context(), s.path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		writeCappedTailJSON(w, entries, maxPayload)
	default:
		http.Error(w, "unsupported usage query", http.StatusForbidden)
	}
}

func writeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeCappedTailJSON[T any](w http.ResponseWriter, items []T, maxBytes int64) {
	if items == nil {
		items = []T{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		http.Error(w, "marshal usage history", http.StatusInternalServerError)
		return
	}
	truncated := false
	if maxBytes > 0 && int64(len(raw)) > maxBytes {
		lo, hi := 0, len(items)-1
		best := len(items)
		var bestRaw []byte
		for lo <= hi {
			mid := lo + (hi-lo)/2
			candidate, err := json.Marshal(items[mid:])
			if err != nil {
				http.Error(w, "marshal usage history", http.StatusInternalServerError)
				return
			}
			if int64(len(candidate)) <= maxBytes {
				best = mid
				bestRaw = candidate
				hi = mid - 1
			} else {
				lo = mid + 1
			}
		}
		if best >= len(items) {
			http.Error(w, server.ErrRunnerPayloadTooLarge.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		truncated = best > 0
		raw = bestRaw
	}
	if truncated {
		w.Header().Set("X-Harnez-Truncated", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

func (s *telemetryCollectorService) Run(ctx context.Context) error {
	// Shutdown stops every armed Claude compact fill timer (issue 1303 M3). Pending
	// records stay on disk; start-up prunes the stale and corrupt ones first.
	defer server.StopClaudeFills()
	if err := server.PruneClaudeFillPending(s.path, time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "harnez server: prune compact fills: %v\n", err)
	}
	if err := s.drainSpool(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "harnez server: drain telemetry spool: %v\n", err)
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.drainSpool(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "harnez server: drain telemetry spool: %v\n", err)
			}
		}
	}
}

func (s *telemetryCollectorService) drainSpool(ctx context.Context) error {
	dir, err := s.config.TelemetrySpoolDir()
	if err != nil {
		return err
	}
	return server.DrainTelemetrySpool(ctx, dir, func(_ context.Context, data []byte) error {
		var record server.TelemetryRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return errors.Join(err, server.ErrQuarantine)
		}
		err := processTelemetryRecord(record, s.path)
		if err != nil {
			if s.shouldRateLimitWarning(err) {
				return nil
			}
			if isPermanentTelemetryError(err) {
				return errors.Join(err, server.ErrQuarantine)
			}
		}
		return err
	})
}

func (s *telemetryCollectorService) shouldRateLimitWarning(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.Contains(msg, "telemetry: no tool call found for session") {
		return false
	}
	s.warnMu.Lock()
	defer s.warnMu.Unlock()
	if s.warnedRecord == nil {
		s.warnedRecord = make(map[string]time.Time)
	}
	limit := s.config.TelemetryWarningRateLimit
	if limit <= 0 {
		limit = time.Minute
	}
	now := time.Now()
	if last, ok := s.warnedRecord[msg]; ok && now.Sub(last) < limit {
		return true
	}
	s.warnedRecord[msg] = now
	fmt.Fprintf(os.Stderr, "harnez server: %v\n", err)
	return true
}

func isPermanentTelemetryError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "constraint") || strings.Contains(msg, "schema missing")
}

func processTelemetryRecord(record server.TelemetryRecord, dbPath string) error {
	_, err := processTelemetryRecordResult(context.Background(), record, dbPath)
	return err
}

func processTelemetryRecordResult(ctx context.Context, record server.TelemetryRecord, dbPath string) ([]byte, error) {
	switch record.Kind {
	case "tool_call", "agy_post_tool", "codex", "agy_compaction", "cli_invocation", "issue_snapshot":
		err := server.WithTelemetryReceipt(dbPath, record.RecordID, func(tx *telemetry.DB) error { return processTelemetryWrite(record, tx) })
		if err == nil && record.Kind == "codex" {
			// The quota sample runs after the record's commit (issue 1303 M2).
			scheduleClaudeCompactQuota(dbPath, record)
		}
		return nil, err
	}
	switch record.Kind {
	case "ensure_schema":
		version, migrations, err := server.EnsureTelemetrySchema(dbPath)
		if err != nil {
			return nil, err
		}
		if len(migrations) == 0 {
			return nil, nil
		}
		return json.Marshal(struct {
			Version    int      `json:"version"`
			Migrations []string `json:"migrations"`
		}{version, migrations})
	case "merge_legacy":
		var request struct {
			DBPath  string    `json:"db_path"`
			Sources []string  `json:"sources"`
			Now     time.Time `json:"now"`
		}
		if err := json.Unmarshal(record.Payload, &request); err != nil {
			return nil, err
		}
		if request.DBPath != "" {
			dbPath = request.DBPath
		}
		output, err := server.MergeLegacyToolCalls(ctx, dbPath, request.Sources, request.Now)
		if err != nil {
			return nil, err
		}
		return json.Marshal(struct {
			Output string `json:"output"`
		}{Output: output})
	default:
		return nil, errors.New("unknown telemetry record kind")
	}
}

func processTelemetryWrite(record server.TelemetryRecord, db *telemetry.DB) error {
	switch record.Kind {
	case "tool_call":
		return server.InsertToolCallPayloadTx(db, record.Payload)
	case "agy_post_tool":
		return server.ProcessAgyPostToolTx(db, record.Payload)
	case "codex":
		return server.ProcessCodexTelemetryTx(db, record.Payload)
	case "agy_compaction":
		var sessionID string
		if err := json.Unmarshal(record.Payload, &sessionID); err != nil {
			return err
		}
		return server.InsertAgyCompactionPairTx(db, sessionID)
	case "cli_invocation":
		var row telemetry.CLIInvocation
		if err := json.Unmarshal(record.Payload, &row); err != nil {
			return err
		}
		return db.InsertCLIInvocation(row)
	case "issue_snapshot":
		var row telemetry.IssueStatusSnapshot
		if err := json.Unmarshal(record.Payload, &row); err != nil {
			return err
		}
		_, err := db.InsertIssueSnapshot(row)
		return err
	}
	return errors.New("unknown telemetry write kind")
}

func sendTelemetryRecord(kind string, payload any) server.UpdateResult {
	return sendTelemetryRecordTo(os.Stderr, kind, payload)
}

func sendTelemetryRecordTo(errOut io.Writer, kind string, payload any) server.UpdateResult {
	config, err := server.LoadConfig()
	if err != nil {
		return server.UpdateDropped
	}
	bodyPayload, err := json.Marshal(payload)
	if err != nil {
		return server.UpdateDropped
	}
	record, err := server.NewTelemetryRecord(kind, bodyPayload)
	if err != nil {
		return server.UpdateDropped
	}
	result := server.DispatchTelemetry(context.Background(), config, record)
	if result == server.UpdateDropped && errOut != nil && (os.Getenv("HARNEZ_DEBUG") != "" || os.Getenv("HARNEZ_TELEMETRY_DEBUG") != "") {
		fmt.Fprintln(errOut, "harnez: telemetry record dropped (broker channel or host spool unavailable/full)")
	}
	return result
}
