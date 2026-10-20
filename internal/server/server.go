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
			client := &http.Client{Timeout: duration(s.Config.HTTPClientTimeout)}
			resp, probeErr := client.Get("http://" + s.Config.Address() + "/status")
			if probeErr == nil {
				defer resp.Body.Close()
				var status struct {
					Health  string `json:"health"`
					Version string `json:"version"`
					Server  string `json:"server"`
				}
				if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&status) == nil && status.Health == "ok" && status.Server == "harnez" && strings.TrimSpace(status.Version) != "" {
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
	mux.HandleFunc("GET /status", statusHandler(s.Version))
	httpServer := &http.Server{Handler: mux, ReadHeaderTimeout: duration(s.Config.ReadHeaderTimeout)}
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), duration(s.Config.ShutdownTimeout))
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
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"health": "ok", "server": "harnez", "version": version})
	}
}
