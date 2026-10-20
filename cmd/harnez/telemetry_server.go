package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"ubunatic.com/harnez/internal/server"
)

type telemetryCollectorService struct {
	config   server.Config
	path     string
	interval time.Duration
}

func (s *telemetryCollectorService) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+s.config.TelemetryPath, s.handleRecord)
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
	if err := processTelemetryRecord(record, s.path); err != nil {
		http.Error(w, "telemetry write failed", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *telemetryCollectorService) Run(ctx context.Context) error {
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
			return err
		}
		return processTelemetryRecord(record, s.path)
	})
}

func processTelemetryRecord(record server.TelemetryRecord, dbPath string) error {
	switch record.Kind {
	case "tool_call":
		return server.InsertToolCallPayload(dbPath, record.Payload)
	case "agy_post_tool":
		return server.ProcessAgyPostTool(dbPath, record.Payload)
	case "codex":
		return server.ProcessCodexTelemetry(dbPath, record.Payload)
	default:
		return errors.New("unknown telemetry record kind")
	}
}

func sendTelemetryRecord(kind string, payload any) server.UpdateResult {
	config, err := server.LoadConfig()
	if err != nil {
		return server.UpdateDropped
	}
	socket, err := config.Socket()
	if err != nil {
		return server.UpdateDropped
	}
	spoolDir, err := config.TelemetrySpoolDir()
	if err != nil {
		return server.UpdateDropped
	}
	bodyPayload, err := json.Marshal(payload)
	if err != nil {
		return server.UpdateDropped
	}
	body, err := json.Marshal(server.TelemetryRecord{Kind: kind, Payload: bodyPayload})
	if err != nil {
		return server.UpdateDropped
	}
	result := server.PostTelemetry(context.Background(), socket, config.TelemetryPath, body, config.UpdateTimeout, spoolDir, config.TelemetrySpoolMaxBytes)
	if result == server.UpdateDropped {
		fmt.Fprintln(os.Stderr, "harnez: telemetry record dropped (server unavailable and spool unavailable or full)")
	}
	return result
}
