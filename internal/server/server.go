package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
)

type Service interface{ Run(context.Context) error }
type readiness interface{ Ready() bool }

type Server struct {
	Config   Config
	Version  string
	Services []Service
}

func (s Server) Run(ctx context.Context) error {
	if s.Config.Host != "127.0.0.1" {
		return fmt.Errorf("server must bind to 127.0.0.1")
	}
	listener, err := net.Listen("tcp", s.Config.Address())
	if err != nil {
		if isAddressInUse(err) {
			client := &http.Client{Timeout: s.Config.HTTPClientTimeout}
			resp, probeErr := client.Get("http://" + s.Config.Address() + "/status")
			if probeErr == nil {
				defer resp.Body.Close()
				var status struct {
					Health  string `json:"health"`
					Version string `json:"version"`
					Server  string `json:"server"`
				}
				if json.NewDecoder(resp.Body).Decode(&status) == nil && status.Server == "harnez" && strings.TrimSpace(status.Version) != "" && ((resp.StatusCode == http.StatusOK && status.Health == "ok") || (resp.StatusCode == http.StatusServiceUnavailable && status.Health == "starting")) {
					fmt.Println("harnez server already running")
					return nil
				}
			}
			return fmt.Errorf("cannot start harnez server: %s is in use by another program (status probe failed)", s.Config.Address())
		}
		return fmt.Errorf("listen on %s: %w", s.Config.Address(), err)
	}
	defer listener.Close()
	mux := http.NewServeMux()
	ready := func() bool {
		for _, service := range s.Services {
			if service, ok := service.(readiness); ok && !service.Ready() {
				return false
			}
		}
		return true
	}
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) { statusHandlerReady(s.Version, ready()).ServeHTTP(w, r) })
	httpServer := &http.Server{Handler: mux, ReadHeaderTimeout: s.Config.ReadHeaderTimeout}
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.Serve(listener) }()
	serviceCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	serviceErr := make(chan error, len(s.Services))
	for _, service := range s.Services {
		go func(service Service) { serviceErr <- service.Run(serviceCtx) }(service)
	}
	var result error
	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			result = err
		}
	case err := <-serviceErr:
		if err != nil {
			result = err
		}
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.Config.ShutdownTimeout)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil && result == nil {
		result = err
	}
	return result
}

func isAddressInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

func statusHandler(version string) http.HandlerFunc {
	return statusHandlerReady(version, true)
}

func statusHandlerReady(version string, ready bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		health := "ok"
		if !ready {
			health = "starting"
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"health": health, "server": "harnez", "version": version})
	}
}
