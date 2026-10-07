package channel

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
)

// ErrUnknownSession indicates that no endpoint is registered for a session ID.
var ErrUnknownSession = errors.New("unknown channel session")

// ErrStaleSession indicates a dead process or an endpoint that fails identity checks.
var ErrStaleSession = errors.New("stale channel session")

// Endpoint is public discovery information; the authentication token stays on disk.
type Endpoint struct {
	SessionID  string `json:"session_id"`
	PID        int    `json:"pid"`
	URL        string `json:"url"`
	WorkingDir string `json:"working_dir"`
}

type registration struct {
	Endpoint
	Token string `json:"token"`
}

func registryPath(cfg Config, sessionID string) string {
	// Hashing permits provider IDs without ever interpreting them as paths.
	return filepath.Join(cfg.RegistryDir, fmt.Sprintf("%x.json", sha256.Sum256([]byte(sessionID))))
}

func readRegistration(cfg Config, path string) (registration, error) {
	file, err := os.Open(path)
	if err != nil {
		return registration{}, err
	}
	defer file.Close()
	var reg registration
	if err := json.NewDecoder(io.LimitReader(file, int64(cfg.MaxRPCBytes))).Decode(&reg); err != nil {
		return reg, err
	}
	if reg.SessionID == "" || reg.PID <= 0 || reg.Token == "" || registryPath(cfg, reg.SessionID) != path {
		return reg, fmt.Errorf("invalid channel registration: %s", path)
	}
	u, err := url.Parse(reg.URL)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Port() == "" || u.Port() == "0" {
		return reg, fmt.Errorf("invalid channel endpoint: %s", path)
	}
	if err := validateLoopback(u.Host); err != nil {
		return reg, err
	}
	return reg, nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Windows cannot signal zero; its authenticated health check remains mandatory.
	if runtime.GOOS == "windows" {
		return true
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer process.Release()
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

func channelClient(cfg Config) *http.Client {
	return &http.Client{
		Timeout: cfg.timeout(),
		// No proxy or redirect can move a token or event off the loopback endpoint.
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func checkRegistration(ctx context.Context, cfg Config, reg registration) error {
	if !processAlive(reg.PID) {
		return fmt.Errorf("%w: %s (dead pid %d)", ErrStaleSession, reg.SessionID, reg.PID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reg.URL+"/health", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+reg.Token)
	resp, err := channelClient(cfg).Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrStaleSession, reg.SessionID, err)
	}
	defer resp.Body.Close()
	var endpoint Endpoint
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, int64(cfg.MaxRPCBytes))).Decode(&endpoint) != nil || endpoint != reg.Endpoint {
		return fmt.Errorf("%w: %s (endpoint identity mismatch)", ErrStaleSession, reg.SessionID)
	}
	return nil
}

func lookup(ctx context.Context, cfg Config, sessionID string) (registration, error) {
	reg, err := readRegistration(cfg, registryPath(cfg, sessionID))
	if errors.Is(err, os.ErrNotExist) {
		return reg, fmt.Errorf("%w: %s", ErrUnknownSession, sessionID)
	}
	if err != nil {
		return reg, err
	}
	return reg, checkRegistration(ctx, cfg, reg)
}

// List returns only live endpoints whose authenticated identity matches the registry.
// Stale entries are ignored, not deleted by discovery; a restart can replace them.
func List(ctx context.Context, cfg Config) ([]Endpoint, error) {
	endpoints := make([]Endpoint, 0)
	entries, err := os.ReadDir(cfg.RegistryDir)
	if errors.Is(err, os.ErrNotExist) {
		return endpoints, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		reg, err := readRegistration(cfg, filepath.Join(cfg.RegistryDir, entry.Name()))
		if err == nil && checkRegistration(ctx, cfg, reg) == nil {
			endpoints = append(endpoints, reg.Endpoint)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].SessionID < endpoints[j].SessionID })
	return endpoints, nil
}

func register(ctx context.Context, cfg Config, reg registration) error {
	if err := os.MkdirAll(cfg.RegistryDir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(cfg.RegistryDir, 0700); err != nil {
		return err
	}
	path := registryPath(cfg, reg.SessionID)
	if previous, err := readRegistration(cfg, path); err == nil {
		if err := checkRegistration(ctx, cfg, previous); err == nil {
			return fmt.Errorf("channel session already registered: %s", reg.SessionID)
		}
		if err := unregister(cfg, previous); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing channel registration: %w", err)
	}
	file, err := os.CreateTemp(cfg.RegistryDir, ".channel-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := json.NewEncoder(file).Encode(reg); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	// Publish a complete file atomically without replacing another live registrar.
	return os.Link(file.Name(), path)
}

func unregister(cfg Config, reg registration) error {
	path := registryPath(cfg, reg.SessionID)
	current, err := readRegistration(cfg, path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// An old process must not remove a replacement's registration on exit.
	if current.Token != reg.Token {
		return nil
	}
	return os.Remove(path)
}
