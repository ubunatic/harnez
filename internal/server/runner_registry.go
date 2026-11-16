package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ubunatic.com/harnez/internal/procs"
)

const (
	RunnerOpOwnRegister = "own_register"
	RunnerOpOwnUpdate   = "own_update"
	RunnerOpOwnQuery    = "own_query"
)

// RunnerLaunchBinding pins a host-launched broker socketpair to a single
// launch identity (and, once bound, one session ID and child process tree).
type RunnerLaunchBinding struct {
	opMu           sync.Mutex
	mu             sync.Mutex
	LaunchID       string
	SessionID      string
	BoundPID       int
	BoundStarttime uint64
	bindReady      chan struct{}
}

// NewRunnerLaunchBinding mints a fresh cryptographic launch identity for one
// child broker socketpair.
func NewRunnerLaunchBinding(sessionID string) (*RunnerLaunchBinding, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, fmt.Errorf("generate launch identity: %w", err)
	}
	return &RunnerLaunchBinding{
		LaunchID:  hex.EncodeToString(raw[:]),
		SessionID: strings.TrimSpace(sessionID),
	}, nil
}

// RequireChildPIDBind makes registry operations wait until BindChildPID runs,
// closing the race window between cmd.Start and BindChildPID.
func (b *RunnerLaunchBinding) RequireChildPIDBind() {
	b.mu.Lock()
	if b.bindReady == nil && b.BoundPID == 0 {
		b.bindReady = make(chan struct{})
	}
	b.mu.Unlock()
}

// BindChildPID records the started child's PID and proc starttime on the host
// side so later registry RPCs cannot spoof foreign PIDs.
func (b *RunnerLaunchBinding) BindChildPID(pid int) {
	if b == nil || pid <= 0 {
		return
	}
	start := procs.ProcessStarttime(pid)
	b.mu.Lock()
	b.BoundPID = pid
	b.BoundStarttime = start
	ch := b.bindReady
	b.bindReady = nil
	b.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (b *RunnerLaunchBinding) waitForBind(ctx context.Context) error {
	b.mu.Lock()
	ch := b.bindReady
	b.mu.Unlock()
	if ch == nil {
		return nil
	}
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *RunnerLaunchBinding) snapshot() (launchID, sessionID string, boundPID int, boundStart uint64) {
	if b == nil {
		return "", "", 0, 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.LaunchID, b.SessionID, b.BoundPID, b.BoundStarttime
}

func (b *RunnerLaunchBinding) commitSessionID(id string) {
	b.mu.Lock()
	if b.SessionID == "" {
		b.SessionID = id
	}
	b.mu.Unlock()
}

// RunnerRegistryRequest is the strict JSON payload accepted on the runner
// channel for own_register, own_update, and own_query. Arbitrary filesystem
// paths, parent/caller PIDs, and cross-session selectors are structurally absent
// and rejected by strict decoding.
type RunnerRegistryRequest struct {
	SessionID         string `json:"session_id,omitempty"`
	Name              string `json:"name,omitempty"`
	Provider          string `json:"provider,omitempty"`
	Model             string `json:"model,omitempty"`
	Tier              string `json:"tier,omitempty"`
	Role              string `json:"role,omitempty"`
	Status            string `json:"status,omitempty"`
	ProcessPID        int    `json:"process_pid,omitempty"`
	ProcessStarttime  uint64 `json:"process_starttime,omitempty"`
	ProviderPID       int    `json:"provider_pid,omitempty"`
	ProviderStarttime uint64 `json:"provider_starttime,omitempty"`
	ProviderSessionID string `json:"provider_session_id,omitempty"`
	Turn              int    `json:"turn,omitempty"`
	LastError         string `json:"last_error,omitempty"`
	Response          string `json:"response,omitempty"`
}

// RunnerRegistryRelayPayload is sent internally by the trusted host broker over
// Config.Socket to Config.RegistryPath with host-authenticated binding fields.
type RunnerRegistryRelayPayload struct {
	Action         string                `json:"action"`
	LaunchID       string                `json:"launch_id"`
	SessionID      string                `json:"session_id"`
	BoundPID       int                   `json:"bound_pid,omitempty"`
	BoundStarttime uint64                `json:"bound_starttime,omitempty"`
	Request        RunnerRegistryRequest `json:"request"`
}

// SendRunnerRegistry sends one own-session registry operation over the
// inherited runner broker FD and decodes resp.Result into out.
func SendRunnerRegistry(ctx context.Context, cfg Config, op string, req RunnerRegistryRequest, out any) error {
	switch op {
	case RunnerOpOwnRegister, RunnerOpOwnUpdate, RunnerOpOwnQuery:
	default:
		return fmt.Errorf("%w: unsupported registry operation %q", ErrRunnerForbidden, op)
	}
	rawPayload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	resp, err := SendRunnerRequest(ctx, cfg, RunnerRequest{
		Op:      op,
		Payload: rawPayload,
	})
	if err != nil {
		return err
	}
	if resp.Status < 200 || resp.Status >= 300 {
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
	if out == nil || len(resp.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		return fmt.Errorf("decode runner registry response: %w", err)
	}
	return nil
}

func hostSocketRegistry(hostSocket string, cfg Config) func(context.Context, RunnerRegistryRelayPayload) ([]byte, error) {
	return func(callCtx context.Context, relay RunnerRegistryRelayPayload) ([]byte, error) {
		body, err := json.Marshal(relay)
		if err != nil {
			return nil, err
		}
		result, err := PostTelemetryCommand(callCtx, hostSocket, cfg.RegistryPath, body, cfg.HTTPClientTimeout)
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

func handleRunnerRegistry(ctx context.Context, cfg Config, op string, payload json.RawMessage, binding *RunnerLaunchBinding, fn func(context.Context, RunnerRegistryRelayPayload) ([]byte, error)) RunnerResponse {
	if fn == nil || binding == nil {
		return RunnerResponse{Status: http.StatusServiceUnavailable, Error: ErrRunnerUnavailable.Error()}
	}
	var req RunnerRegistryRequest
	if err := decodeStrictJSON(payload, &req); err != nil {
		return RunnerResponse{Status: http.StatusBadRequest, Error: "invalid runner registry request"}
	}
	if req.Turn < 0 {
		return RunnerResponse{Status: http.StatusForbidden, Error: ErrRunnerForbidden.Error()}
	}
	if err := validateRegistryRequestFields(req); err != nil {
		return RunnerResponse{Status: http.StatusForbidden, Error: err.Error()}
	}
	if err := binding.waitForBind(ctx); err != nil {
		return RunnerResponse{Status: http.StatusServiceUnavailable, Error: ErrRunnerUnavailable.Error()}
	}

	binding.opMu.Lock()
	defer binding.opMu.Unlock()

	launchID, boundSessionID, boundPID, boundStart := binding.snapshot()
	if strings.TrimSpace(launchID) == "" {
		return RunnerResponse{Status: http.StatusForbidden, Error: "missing host launch identity"}
	}
	if err := validateRegistryPIDs(req, boundPID, boundStart); err != nil {
		return RunnerResponse{Status: http.StatusForbidden, Error: err.Error()}
	}

	targetID, err := authorizeRegistrySessionID(cfg, op, req.SessionID, boundSessionID)
	if err != nil {
		return RunnerResponse{Status: http.StatusForbidden, Error: err.Error()}
	}
	req.SessionID = targetID

	relay := RunnerRegistryRelayPayload{
		Action:         op,
		LaunchID:       launchID,
		SessionID:      targetID,
		BoundPID:       boundPID,
		BoundStarttime: boundStart,
		Request:        req,
	}
	raw, err := fn(ctx, relay)
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
	if op == RunnerOpOwnRegister {
		binding.commitSessionID(targetID)
	}
	maxPayload := cfg.RunnerChannel.MaxResultPayloadBytes()
	if maxPayload <= 0 || int64(len(raw)) > maxPayload {
		return RunnerResponse{Status: http.StatusRequestEntityTooLarge, Error: ErrRunnerPayloadTooLarge.Error()}
	}
	return RunnerResponse{Status: http.StatusOK, Result: json.RawMessage(raw)}
}

func authorizeRegistrySessionID(cfg Config, op, reqID, boundID string) (string, error) {
	reqID = strings.TrimSpace(reqID)
	switch op {
	case RunnerOpOwnRegister:
		if !validSafeRegistryID(reqID, cfg.RunnerChannel.RegistryMaxIDLen) {
			return "", fmt.Errorf("%w: invalid session_id for registration", ErrRunnerForbidden)
		}
		if boundID != "" && reqID != boundID {
			return "", fmt.Errorf("%w: session_id %q does not match launch identity", ErrRunnerForbidden, reqID)
		}
		return reqID, nil
	case RunnerOpOwnUpdate, RunnerOpOwnQuery:
		if boundID == "" {
			return "", fmt.Errorf("%w: no launch-bound session registered", ErrRunnerForbidden)
		}
		if reqID != "" && reqID != boundID {
			return "", fmt.Errorf("%w: cross-session %s rejected", ErrRunnerForbidden, op)
		}
		return boundID, nil
	default:
		return "", fmt.Errorf("%w: unsupported registry op %q", ErrRunnerForbidden, op)
	}
}

func validateRegistryRequestFields(req RunnerRegistryRequest) error {
	switch req.Status {
	case "", "running", "completed", "failed", "stopped":
	default:
		return fmt.Errorf("%w: unsupported runner session status %q", ErrRunnerForbidden, req.Status)
	}
	switch strings.ToLower(strings.TrimSpace(req.Role)) {
	case "", "worker", "developer", "researcher", "reviewer", "explorer":
	default:
		return fmt.Errorf("%w: unsupported runner role %q", ErrRunnerForbidden, req.Role)
	}
	return nil
}

func validSafeRegistryID(id string, maxLen int) bool {
	if id == "" || (maxLen > 0 && len(id) > maxLen) {
		return false
	}
	if strings.ContainsAny(id, "/\\ \t\r\n\x00") || id == "." || id == ".." || filepath.Base(id) != id {
		return false
	}
	return true
}

func validateRegistryPIDs(req RunnerRegistryRequest, boundPID int, boundStart uint64) error {
	if req.ProcessPID < 0 || req.ProviderPID < 0 {
		return fmt.Errorf("%w: negative PID rejected", ErrRunnerForbidden)
	}
	if req.ProcessPID > 0 {
		if err := validateOneProcessPID(req.ProcessPID, req.ProcessStarttime, boundPID, boundStart, false); err != nil {
			return err
		}
	}
	if req.ProviderPID > 0 {
		if err := validateOneProcessPID(req.ProviderPID, req.ProviderStarttime, boundPID, boundStart, true); err != nil {
			return err
		}
	}
	return nil
}

func validateOneProcessPID(pid int, starttime uint64, boundPID int, boundStart uint64, allowDescendant bool) error {
	if boundPID <= 0 {
		return fmt.Errorf("%w: cannot verify PID %d without host-bound child PID", ErrRunnerForbidden, pid)
	}
	if pid <= 1 || pid == os.Getpid() || starttime == 0 {
		return fmt.Errorf("%w: forged or unverified PID %d", ErrRunnerForbidden, pid)
	}
	liveStart := procs.ProcessStarttime(pid)
	if liveStart == 0 || liveStart != starttime {
		return fmt.Errorf("%w: PID %d dead or starttime mismatch", ErrRunnerForbidden, pid)
	}
	if pid == boundPID {
		if boundStart > 0 && starttime != boundStart {
			return fmt.Errorf("%w: bound PID starttime mismatch", ErrRunnerForbidden)
		}
		return nil
	}
	if !allowDescendant {
		return fmt.Errorf("%w: PID %d does not match bound process %d", ErrRunnerForbidden, pid, boundPID)
	}
	members, err := procs.TreeMembers([]procs.TreeRoot{{PID: boundPID, Starttime: boundStart}}, nil, nil)
	if err != nil {
		return fmt.Errorf("%w: inspect launch process tree: %v", ErrRunnerForbidden, err)
	}
	for _, member := range members {
		if member.PID == pid && member.Starttime != 0 && member.Starttime == starttime {
			return nil
		}
	}
	return fmt.Errorf("%w: PID %d is outside launch process tree %d", ErrRunnerForbidden, pid, boundPID)
}
