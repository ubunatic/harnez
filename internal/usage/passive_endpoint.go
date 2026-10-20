package usage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

type PassiveUpdateServer struct {
	Path         string
	Requests     chan<- PassiveRequest
	Timeout      time.Duration
	MaxBodyBytes int64
}

func (s PassiveUpdateServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+s.Path, s.handlePassive)
}

func (s PassiveUpdateServer) handlePassive(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxBodyBytes)
	var payload struct {
		Windows []usagestore.Window `json:"windows"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&payload); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if len(payload.Windows) == 0 {
		http.Error(w, "windows required", http.StatusBadRequest)
		return
	}
	for _, window := range payload.Windows {
		if window.Provider == "" || window.Key == "" || window.Source == "" || window.ObservedAt.IsZero() || window.UsedFraction < 0 || window.UsedFraction > 1 {
			http.Error(w, "invalid window", http.StatusBadRequest)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.Timeout)
	defer cancel()
	done := make(chan error, 1)
	request := PassiveRequest{Windows: payload.Windows, Done: done}
	select {
	case s.Requests <- request:
	case <-ctx.Done():
		http.Error(w, "collector unavailable", http.StatusServiceUnavailable)
		return
	}
	select {
	case err := <-done:
		if err != nil {
			http.Error(w, "collector write failed", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case <-ctx.Done():
		http.Error(w, "collector unavailable", http.StatusServiceUnavailable)
	}
}
