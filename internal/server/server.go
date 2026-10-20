package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

type Service interface{ Run(context.Context) error }
type readiness interface{ Ready() bool }
type RouteRegistrar interface{ RegisterRoutes(*http.ServeMux) }

type Server struct {
	Config        Config
	Version       string
	BuildIdentity string
	Services      []Service
	Output        io.Writer
}

type statusInfo struct {
	Health        string `json:"health"`
	Server        string `json:"server"`
	Version       string `json:"version"`
	BuildIdentity string `json:"build_identity"`
	PID           int    `json:"pid"`
}

func CaptureBuildIdentity() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return info.ModTime().UTC().Format(time.RFC3339Nano)
}

func (s Server) Run(ctx context.Context) error {
	socket, err := s.Config.Socket()
	if err != nil {
		return err
	}
	identity := s.BuildIdentity
	if identity == "" {
		identity = CaptureBuildIdentity()
	}
	if err := s.claimExisting(ctx, socket, identity); err != nil {
		if errors.Is(err, errAlreadyRunning) {
			return nil
		}
		return err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", socket, err)
	}
	defer listener.Close()
	if err := os.Chmod(socket, 0600); err != nil {
		return err
	}
	mux := http.NewServeMux()
	ready := func() bool {
		for _, service := range s.Services {
			if service, ok := service.(readiness); ok && !service.Ready() {
				return false
			}
		}
		return true
	}
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) { writeStatus(w, s.Version, identity, ready()) })
	shutdown := make(chan struct{}, 1)
	mux.HandleFunc("POST /shutdown", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		select {
		case shutdown <- struct{}{}:
		default:
		}
	})
	for _, service := range s.Services {
		if registrar, ok := service.(RouteRegistrar); ok {
			registrar.RegisterRoutes(mux)
		}
	}
	httpServer := &http.Server{Handler: mux, ReadHeaderTimeout: s.Config.ReadHeaderTimeout, ReadTimeout: s.Config.UpdateTimeout}
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
	case <-shutdown:
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

func (s Server) claimExisting(ctx context.Context, socket, identity string) error {
	if _, err := os.Lstat(socket); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	client := UnixHTTPClient(socket, s.Config.HTTPClientTimeout)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/status", nil)
	resp, err := client.Do(req)
	if err != nil {
		return removeStaleSocket(socket)
	}
	defer resp.Body.Close()
	var status statusInfo
	if json.NewDecoder(resp.Body).Decode(&status) != nil || status.Server != "harnez" || strings.TrimSpace(status.Version) == "" || status.BuildIdentity == "" || (resp.StatusCode == http.StatusOK && status.Health != "ok") || (resp.StatusCode == http.StatusServiceUnavailable && status.Health != "starting") || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable) {
		return fmt.Errorf("cannot start harnez server: %s is in use by another program", socket)
	}
	if compareBuild(s.Version, identity, status.Version, status.BuildIdentity) <= 0 {
		out := s.Output
		if out == nil {
			out = os.Stdout
		}
		fmt.Fprintf(out, "already running (v%s)\n", status.Version)
		return errAlreadyRunning
	}
	shutdownReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/shutdown", nil)
	shutdownResp, err := client.Do(shutdownReq)
	if err != nil {
		return fmt.Errorf("request old harnez server shutdown: %w", err)
	}
	shutdownResp.Body.Close()
	if shutdownResp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("old harnez server rejected shutdown: HTTP %d", shutdownResp.StatusCode)
	}
	timer := time.NewTimer(s.Config.TakeoverTimeout)
	defer timer.Stop()
	for {
		if _, err := os.Lstat(socket); errors.Is(err, os.ErrNotExist) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("old harnez server did not release socket within %s", s.Config.TakeoverTimeout)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

var errAlreadyRunning = errors.New("already running")

func removeStaleSocket(socket string) error {
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale server socket: %w", err)
	}
	return nil
}

func compareBuild(versionA, identityA, versionB, identityB string) int {
	a, b := versionA, versionB
	if !strings.HasPrefix(a, "v") {
		a = "v" + a
	}
	if !strings.HasPrefix(b, "v") {
		b = "v" + b
	}
	if semver.IsValid(a) && semver.IsValid(b) {
		if cmp := semver.Compare(a, b); cmp != 0 {
			return cmp
		}
	}
	if identityA > identityB {
		return 1
	}
	if identityA < identityB {
		return -1
	}
	return 0
}

func writeStatus(w http.ResponseWriter, version, identity string, ready bool) {
	w.Header().Set("Content-Type", "application/json")
	code, health := http.StatusOK, "ok"
	if !ready {
		code, health = http.StatusServiceUnavailable, "starting"
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(statusInfo{Health: health, Server: "harnez", Version: version, BuildIdentity: identity, PID: os.Getpid()})
}

func statusHandler(version string) http.HandlerFunc { return statusHandlerReady(version, true) }
func statusHandlerReady(version string, ready bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { writeStatus(w, version, "test-identity", ready) }
}
