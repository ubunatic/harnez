package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/subagent"
)

func (s *telemetryCollectorService) handleRegistry(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.config.UpdateMaxBodyBytes)
	var relay server.RunnerRegistryRelayPayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&relay); err != nil {
		http.Error(w, "invalid registry payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(relay.LaunchID) == "" || strings.TrimSpace(relay.SessionID) == "" {
		http.Error(w, "missing launch or session identity", http.StatusForbidden)
		return
	}
	dir := s.registryDir
	if dir == "" {
		dir = subagent.DefaultStoreDir()
	}
	store, err := subagent.NewHostSessionStoreDirect(dir)
	if err != nil {
		http.Error(w, "open session registry failed", http.StatusServiceUnavailable)
		return
	}

	switch relay.Action {
	case server.RunnerOpOwnRegister:
		s.handleRegistryCreate(w, store, relay)
	case server.RunnerOpOwnUpdate:
		s.handleRegistryUpdate(w, store, relay)
	case server.RunnerOpOwnQuery:
		s.handleRegistryQuery(w, store, relay)
	default:
		http.Error(w, "unsupported registry operation", http.StatusForbidden)
	}
}

func (s *telemetryCollectorService) handleRegistryCreate(w http.ResponseWriter, store *subagent.FileSessionStore, relay server.RunnerRegistryRelayPayload) {
	name := strings.TrimSpace(relay.Request.Name)
	if name == "" {
		name = relay.SessionID
	}
	now := time.Now().UTC()
	sess := &subagent.Session{
		ID:                relay.SessionID,
		LaunchID:          relay.LaunchID,
		Name:              name,
		Provider:          relay.Request.Provider,
		Model:             relay.Request.Model,
		Tier:              relay.Request.Tier,
		Role:              relay.Request.Role,
		HarnessType:       "runner",
		Status:            defaultStatus(relay.Request.Status),
		ProcessPID:        relay.Request.ProcessPID,
		ProcessStarttime:  relay.Request.ProcessStarttime,
		ProviderPID:       relay.Request.ProviderPID,
		ProviderStarttime: relay.Request.ProviderStarttime,
		ProviderSessionID: relay.Request.ProviderSessionID,
		Turn:              relay.Request.Turn,
		LastError:         relay.Request.LastError,
		Response:          relay.Request.Response,
		CreatedAt:         now,
		LastActiveAt:      now,
	}
	if err := store.Create(sess); err != nil {
		if !errors.Is(err, subagent.ErrSessionNameInUse) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		existing, getErr := store.Get(relay.SessionID)
		if getErr != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if existing.LaunchID == "" || existing.LaunchID != relay.LaunchID {
			http.Error(w, "session launch identity mismatch or legacy record", http.StatusForbidden)
			return
		}
		writeJSONResponse(w, existing)
		return
	}
	writeJSONResponse(w, sess)
}

func (s *telemetryCollectorService) handleRegistryUpdate(w http.ResponseWriter, store *subagent.FileSessionStore, relay server.RunnerRegistryRelayPayload) {
	existing, err := store.Get(relay.SessionID)
	if err != nil {
		status := http.StatusServiceUnavailable
		if os.IsNotExist(err) || strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		http.Error(w, "session not found", status)
		return
	}
	// M13: fail closed on legacy missing-identity records or mismatched launch ID.
	if existing.LaunchID == "" || existing.LaunchID != relay.LaunchID {
		http.Error(w, "session lacks matching launch identity", http.StatusForbidden)
		return
	}
	req := relay.Request
	if req.Status != "" {
		existing.Status = req.Status
	}
	if req.ProviderSessionID != "" {
		existing.ProviderSessionID = req.ProviderSessionID
	}
	if req.ProviderPID > 0 {
		existing.ProviderPID = req.ProviderPID
		existing.ProviderStarttime = req.ProviderStarttime
	}
	if req.Turn > existing.Turn {
		existing.Turn = req.Turn
	}
	if req.LastError != "" {
		existing.LastError = req.LastError
	}
	if req.Response != "" {
		existing.Response = req.Response
	}
	existing.LastActiveAt = time.Now().UTC()
	if err := store.Save(existing); err != nil {
		http.Error(w, "save session failed", http.StatusServiceUnavailable)
		return
	}
	writeJSONResponse(w, existing)
}

func (s *telemetryCollectorService) handleRegistryQuery(w http.ResponseWriter, store *subagent.FileSessionStore, relay server.RunnerRegistryRelayPayload) {
	existing, err := store.Get(relay.SessionID)
	if err != nil {
		status := http.StatusServiceUnavailable
		if os.IsNotExist(err) || strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		http.Error(w, "session not found", status)
		return
	}
	if existing.LaunchID == "" || existing.LaunchID != relay.LaunchID {
		http.Error(w, "session lacks matching launch identity", http.StatusForbidden)
		return
	}
	writeJSONResponse(w, existing)
}

func defaultStatus(s string) string {
	if strings.TrimSpace(s) == "" {
		return "running"
	}
	return s
}
