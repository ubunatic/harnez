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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"ubunatic.com/harnez/internal/sessioncontract"
)

func init() {
	// Mark any valid inherited broker FD close-on-exec immediately at startup so
	// unrelated subprocesses never inherit it unless PrepareCmdRunnerBroker
	// intentionally forwards a dup via ExtraFiles.
	if cfg, err := LoadConfig(); err == nil {
		_, _ = parseAndValidateRunnerFD(cfg)
	}
}

const (
	RunnerOpTelemetryIngest = "telemetry_ingest"
	RunnerOpTelemetryReplay = "telemetry_replay"
	RunnerOpUsageRead       = "usage_read"
)

var (
	// ErrInvalidRunnerChannel signals a missing, non-socket, or unconnected broker FD.
	ErrInvalidRunnerChannel = errors.New("runner broker channel unavailable or invalid")
	// ErrRunnerForbidden signals an operation disallowed on the runner channel.
	ErrRunnerForbidden = errors.New("operation not permitted on runner channel")
	// ErrRunnerPayloadTooLarge signals a frame or batch exceeding configured caps.
	ErrRunnerPayloadTooLarge = errors.New("runner channel payload exceeds limit")
	// ErrRunnerUnavailable signals the host collector could not accept records right now.
	ErrRunnerUnavailable = errors.New("host telemetry collector unavailable")
)

type RunnerRequest struct {
	Op      string          `json:"op"`
	Route   string          `json:"route,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type RunnerResponse struct {
	Status int             `json:"status"`
	Error  string          `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

type RunnerReplayBatch struct {
	Records []TelemetryRecord `json:"records"`
}

type RunnerReplayResult struct {
	Results []RunnerReplayRecordResult `json:"results"`
}

type RunnerReplayRecordResult struct {
	Status int    `json:"status"`
	Error  string `json:"error,omitempty"`
}

// SocketPair creates a connected AF_UNIX SOCK_SEQPACKET broker socket pair for
// passing per-request stream connections from a runner via SCM_RIGHTS.
func SocketPair() (hostEnd, runnerEnd *os.File, err error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("create runner socketpair: %w", err)
	}
	return os.NewFile(uintptr(fds[0]), "harnez-broker-host"), os.NewFile(uintptr(fds[1]), "harnez-broker-runner"), nil
}

// SendRunnerRequest opens a fresh ephemeral stream socketpair, hands one end to
// the host broker atomically over HARNEZ_BROKER_FD via SCM_RIGHTS, and executes
// one request/response round-trip without sharing socket state across callers.
func SendRunnerRequest(ctx context.Context, cfg Config, req RunnerRequest) (RunnerResponse, error) {
	fd, err := parseAndValidateRunnerFD(cfg)
	if err != nil {
		return RunnerResponse{}, err
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return RunnerResponse{}, err
	}
	if int64(len(raw)) > cfg.RunnerChannel.MaxFrameBytes {
		return RunnerResponse{}, ErrRunnerPayloadTooLarge
	}

	conn, err := openPerRequestStream(ctx, fd, cfg.HTTPClientTimeout)
	if err != nil {
		return RunnerResponse{}, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else if cfg.HTTPClientTimeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(cfg.HTTPClientTimeout))
	}
	if err := writeChannelFrame(conn, raw, cfg.RunnerChannel.MaxFrameBytes); err != nil {
		return RunnerResponse{}, err
	}
	respBytes, err := readChannelFrame(conn, cfg.RunnerChannel.MaxFrameBytes)
	if err != nil {
		return RunnerResponse{}, err
	}
	var resp RunnerResponse
	if err := decodeStrictJSON(respBytes, &resp); err != nil {
		return RunnerResponse{}, fmt.Errorf("decode runner response: %w", err)
	}
	return resp, nil
}

func openPerRequestStream(ctx context.Context, brokerFD int, fallbackTimeout time.Duration) (net.Conn, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("create per-request socketpair: %w", err)
	}
	clientFile := os.NewFile(uintptr(fds[0]), "harnez-runner-req")
	defer clientFile.Close()
	peerFD := fds[1]
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline && fallbackTimeout > 0 {
		deadline = time.Now().Add(fallbackTimeout)
		hasDeadline = true
	}
	for {
		err := syscall.Sendmsg(brokerFD, []byte{1}, syscall.UnixRights(peerFD), nil, syscall.MSG_DONTWAIT|syscall.MSG_NOSIGNAL)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EWOULDBLOCK) {
			_ = syscall.Close(peerFD)
			return nil, fmt.Errorf("%w: sendmsg: %v", ErrRunnerUnavailable, err)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			_ = syscall.Close(peerFD)
			return nil, fmt.Errorf("%w: %w", ErrRunnerUnavailable, ctxErr)
		}
		timeoutMs := -1
		if hasDeadline {
			rem := time.Until(deadline)
			if rem <= 0 {
				_ = syscall.Close(peerFD)
				return nil, fmt.Errorf("%w: sendmsg timed out", ErrRunnerUnavailable)
			}
			timeoutMs = int((rem + time.Millisecond - 1) / time.Millisecond)
		}
		if ctx.Done() != nil && (timeoutMs < 0 || timeoutMs > 100) {
			timeoutMs = 100
		}
		_, pollErr := unix.Poll([]unix.PollFd{{Fd: int32(brokerFD), Events: unix.POLLOUT}}, timeoutMs)
		if pollErr != nil && !errors.Is(pollErr, syscall.EINTR) {
			_ = syscall.Close(peerFD)
			return nil, fmt.Errorf("%w: poll: %v", ErrRunnerUnavailable, pollErr)
		}
	}
	_ = syscall.Close(peerFD)
	conn, err := net.FileConn(clientFile)
	if err != nil {
		return nil, fmt.Errorf("%w: wrap stream conn: %v", ErrInvalidRunnerChannel, err)
	}
	return conn, nil
}

// RunnerChannelStatus validates that the inherited broker FD env var points to a
// connected AF_UNIX SOCK_SEQPACKET socket.
func RunnerChannelStatus(cfg Config) error {
	_, err := parseAndValidateRunnerFD(cfg)
	return err
}

// DispatchTelemetry sends one telemetry record through the inherited broker FD
// in runner mode (or whenever a valid broker FD is inherited), or via the host
// Unix socket and host spool otherwise.
func DispatchTelemetry(ctx context.Context, cfg Config, record TelemetryRecord) UpdateResult {
	if RunnerChannelStatus(cfg) == nil {
		if result := SendRunnerTelemetry(ctx, cfg, record); result == UpdateDelivered {
			return result
		}
		// Broker FD is present (sandboxed agent): fall back to server socket only,
		// never touching host spool paths on disk.
		socket, err := cfg.Socket()
		if err != nil {
			return UpdateDropped
		}
		body, err := json.Marshal(record)
		if err != nil {
			return UpdateDropped
		}
		return PostUpdate(ctx, socket, cfg.TelemetryPath, body, cfg.UpdateTimeout)
	}
	socket, err := cfg.Socket()
	if err != nil {
		return UpdateDropped
	}
	spoolDir, err := cfg.TelemetrySpoolDir()
	if err != nil {
		return UpdateDropped
	}
	body, err := json.Marshal(record)
	if err != nil {
		return UpdateDropped
	}
	return PostTelemetry(ctx, socket, cfg.TelemetryPath, body, cfg.UpdateTimeout, spoolDir, cfg.TelemetrySpoolMaxBytes)
}

// SendRunnerTelemetry submits one telemetry record via the runner broker channel,
// spilling to the runner's private spool directory only in runner mode on
// transient collector outages. Missing or invalid broker FDs fail closed.
func SendRunnerTelemetry(ctx context.Context, cfg Config, record TelemetryRecord) UpdateResult {
	if _, err := parseAndValidateRunnerFD(cfg); err != nil {
		return UpdateDropped
	}
	if err := validateRunnerTelemetryRecord(cfg, record); err != nil {
		return UpdateDropped
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return UpdateDropped
	}
	resp, err := SendRunnerRequest(ctx, cfg, RunnerRequest{Op: RunnerOpTelemetryIngest, Payload: payload})
	if err == nil && resp.Status >= 200 && resp.Status < 300 {
		return UpdateDelivered
	}
	if errors.Is(err, ErrInvalidRunnerChannel) || errors.Is(err, ErrRunnerPayloadTooLarge) {
		return UpdateDropped
	}
	if err == nil && resp.Status < 500 {
		return UpdateDropped
	}
	return UpdateDropped
}

// RunnerBrokerHandlers supplies host-side callbacks for allowlisted runner RPCs.
type RunnerBrokerHandlers struct {
	Ingest        func(context.Context, TelemetryRecord) error
	UsageQuery    func(context.Context, RunnerUsageQuery) ([]byte, error)
	Registry      func(context.Context, RunnerRegistryRelayPayload) ([]byte, error)
	LaunchBinding *RunnerLaunchBinding
}

// ServeRunnerBroker listens on the host end of the AF_UNIX SOCK_SEQPACKET broker
// socketpair, accepts per-request SCM_RIGHTS stream connections concurrently, and
// dispatches validated telemetry operations.
func ServeRunnerBroker(ctx context.Context, hostEnd *os.File, cfg Config, ingest func(context.Context, TelemetryRecord) error) error {
	return ServeRunnerBrokerWith(ctx, hostEnd, cfg, RunnerBrokerHandlers{Ingest: ingest})
}

// ServeRunnerBrokerWith listens on hostEnd and dispatches allowlisted runner
// telemetry and read-only query handlers concurrently.
func ServeRunnerBrokerWith(ctx context.Context, hostEnd *os.File, cfg Config, handlers RunnerBrokerHandlers) error {
	if hostEnd == nil {
		return fmt.Errorf("serve runner broker: nil host socket")
	}
	rawConn, err := net.FileConn(hostEnd)
	_ = hostEnd.Close()
	if err != nil {
		return fmt.Errorf("serve runner broker: %w", err)
	}
	seqConn, ok := rawConn.(*net.UnixConn)
	if !ok {
		_ = rawConn.Close()
		return fmt.Errorf("serve runner broker: expected *net.UnixConn")
	}
	return serveUnixBroker(ctx, seqConn, cfg, handlers)
}

// RelayRunnerBroker bridges the host end of the runner broker socketpair to
// the host collector Unix socket without exposing host-only admin operations.
func RelayRunnerBroker(ctx context.Context, hostEnd *os.File, hostSocket string, cfg Config) error {
	binding, err := NewRunnerLaunchBinding("")
	if err != nil {
		_ = hostEnd.Close()
		return err
	}
	return RelayRunnerBrokerWithBinding(ctx, hostEnd, hostSocket, cfg, binding)
}

// RelayRunnerBrokerWithBinding bridges hostEnd to hostSocket using an explicit
// launch-bound identity for registry authorization.
func RelayRunnerBrokerWithBinding(ctx context.Context, hostEnd *os.File, hostSocket string, cfg Config, binding *RunnerLaunchBinding) error {
	return ServeRunnerBrokerWith(ctx, hostEnd, cfg, RunnerBrokerHandlers{
		Ingest:        hostSocketIngestor(hostSocket, cfg),
		UsageQuery:    hostSocketUsageQuery(hostSocket, cfg),
		Registry:      hostSocketRegistry(hostSocket, cfg),
		LaunchBinding: binding,
	})
}

func serveUnixBroker(ctx context.Context, seqConn *net.UnixConn, cfg Config, handlers RunnerBrokerHandlers) error {
	serveCtx, cancelServe := context.WithCancel(ctx)
	done := make(chan struct{})
	var closeOnce sync.Once
	closeSeq := func() { closeOnce.Do(func() { _ = seqConn.Close() }) }
	defer closeSeq()
	go func() {
		select {
		case <-serveCtx.Done():
			closeSeq()
		case <-done:
		}
	}()
	defer close(done)

	var wg sync.WaitGroup
	defer wg.Wait()
	defer cancelServe()

	maxStreams := cfg.RunnerChannel.MaxConcurrentStreams
	if maxStreams <= 0 {
		return fmt.Errorf("serve runner broker: max_concurrent_streams must be positive")
	}
	sem := make(chan struct{}, maxStreams)
	buf := make([]byte, 16)
	oob := make([]byte, syscall.CmsgSpace(4*8))
	for {
		_, oobn, flags, _, err := seqConn.ReadMsgUnix(buf, oob)
		if err != nil {
			if serveCtx.Err() != nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		clientConn, err := extractSCMStreamConn(oob[:oobn], flags)
		if err != nil {
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-serveCtx.Done():
			_ = clientConn.Close()
			return nil
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			defer func() { <-sem }()
			_ = ServeRunnerConnWith(serveCtx, c, cfg, handlers)
		}(clientConn)
	}
}

func extractSCMStreamConn(oob []byte, flags int) (net.Conn, error) {
	scms, parseErr := syscall.ParseSocketControlMessage(oob)
	var fds []int
	for i := range scms {
		if parsed, err := syscall.ParseUnixRights(&scms[i]); err == nil {
			fds = append(fds, parsed...)
		}
	}
	closeAll := func() {
		for _, fd := range fds {
			_ = syscall.Close(fd)
		}
	}
	if flags&syscall.MSG_CTRUNC != 0 || parseErr != nil || len(scms) != 1 || len(fds) != 1 {
		closeAll()
		return nil, fmt.Errorf("invalid scm control message")
	}
	f := os.NewFile(uintptr(fds[0]), "harnez-broker-client")
	defer f.Close()
	if err := validateConnectedUnixSocket(fds[0], syscall.SOCK_STREAM); err != nil {
		return nil, err
	}
	return net.FileConn(f)
}

func isPermanentHTTPStatus(code int) bool {
	if code < 400 || code >= 500 {
		return false
	}
	return code != http.StatusRequestTimeout && code != http.StatusRequestEntityTooLarge && code != http.StatusTooManyRequests
}

func hostSocketIngestor(hostSocket string, cfg Config) func(context.Context, TelemetryRecord) error {
	return func(callCtx context.Context, record TelemetryRecord) error {
		body, err := json.Marshal(record)
		if err != nil {
			return errors.Join(err, ErrQuarantine)
		}
		if _, err := PostTelemetryCommand(callCtx, hostSocket, cfg.TelemetryPath, body, cfg.HTTPClientTimeout); err != nil {
			var statusErr *HTTPStatusError
			if errors.As(err, &statusErr) && isPermanentHTTPStatus(statusErr.StatusCode) {
				return errors.Join(err, ErrQuarantine)
			}
			return fmt.Errorf("%w: %v", ErrRunnerUnavailable, err)
		}
		return nil
	}
}

// ServeRunnerConn handles framed runner telemetry RPCs on a host stream
// connection, enforcing allowlisted operations, payload caps, and strict JSON.
func ServeRunnerConn(ctx context.Context, conn net.Conn, cfg Config, ingest func(context.Context, TelemetryRecord) error) error {
	return ServeRunnerConnWith(ctx, conn, cfg, RunnerBrokerHandlers{Ingest: ingest})
}

// ServeRunnerConnWith handles allowlisted framed runner RPCs using handlers.
func ServeRunnerConnWith(ctx context.Context, conn net.Conn, cfg Config, handlers RunnerBrokerHandlers) error {
	if conn == nil {
		return fmt.Errorf("serve runner conn: nil connection")
	}
	if handlers.Ingest == nil && handlers.UsageQuery == nil && handlers.Registry == nil {
		return fmt.Errorf("serve runner conn: no handlers registered")
	}
	done := make(chan struct{})
	var closeOnce sync.Once
	closeConn := func() { closeOnce.Do(func() { _ = conn.Close() }) }
	defer closeConn()
	go func() {
		select {
		case <-ctx.Done():
			closeConn()
		case <-done:
		}
	}()
	defer close(done)

	for {
		if cfg.HTTPClientTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(cfg.HTTPClientTimeout))
		}
		frame, err := readChannelFrame(conn, cfg.RunnerChannel.MaxFrameBytes)
		if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		if errors.Is(err, ErrRunnerPayloadTooLarge) {
			_ = writeRunnerResponse(conn, cfg.RunnerChannel.MaxFrameBytes, RunnerResponse{Status: http.StatusRequestEntityTooLarge, Error: err.Error()})
			return err
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		resp := handleRunnerFrame(ctx, cfg, frame, handlers)
		if cfg.HTTPClientTimeout > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(cfg.HTTPClientTimeout))
		}
		if err := writeRunnerResponse(conn, cfg.RunnerChannel.MaxFrameBytes, resp); err != nil {
			return err
		}
	}
}

func handleRunnerFrame(ctx context.Context, cfg Config, frame []byte, handlers RunnerBrokerHandlers) RunnerResponse {
	var req RunnerRequest
	if err := decodeStrictJSON(frame, &req); err != nil {
		return RunnerResponse{Status: http.StatusBadRequest, Error: "invalid runner request"}
	}
	if strings.TrimSpace(req.Route) != "" {
		return RunnerResponse{Status: http.StatusForbidden, Error: ErrRunnerForbidden.Error()}
	}
	contract, err := sessioncontract.Load()
	if err != nil {
		return RunnerResponse{Status: http.StatusServiceUnavailable, Error: ErrRunnerUnavailable.Error()}
	}
	if !contract.AllowsRunnerOperation(req.Op) {
		return RunnerResponse{Status: http.StatusForbidden, Error: ErrRunnerForbidden.Error()}
	}
	switch req.Op {
	case RunnerOpTelemetryIngest:
		if handlers.Ingest == nil {
			return RunnerResponse{Status: http.StatusServiceUnavailable, Error: ErrRunnerUnavailable.Error()}
		}
		return handleRunnerIngest(ctx, cfg, req.Payload, handlers.Ingest)
	case RunnerOpTelemetryReplay:
		if handlers.Ingest == nil {
			return RunnerResponse{Status: http.StatusServiceUnavailable, Error: ErrRunnerUnavailable.Error()}
		}
		return handleRunnerReplay(ctx, cfg, req.Payload, handlers.Ingest)
	case RunnerOpUsageRead:
		return handleRunnerUsageRead(ctx, cfg, req.Payload, handlers.UsageQuery)
	case RunnerOpOwnRegister, RunnerOpOwnUpdate, RunnerOpOwnQuery:
		return handleRunnerRegistry(ctx, cfg, req.Op, req.Payload, handlers.LaunchBinding, handlers.Registry)
	default:
		return RunnerResponse{Status: http.StatusForbidden, Error: ErrRunnerForbidden.Error()}
	}
}

func handleRunnerIngest(ctx context.Context, cfg Config, payload json.RawMessage, ingest func(context.Context, TelemetryRecord) error) RunnerResponse {
	var rec TelemetryRecord
	if err := decodeStrictJSON(payload, &rec); err != nil {
		return RunnerResponse{Status: http.StatusBadRequest, Error: "invalid telemetry record"}
	}
	if err := validateRunnerTelemetryRecord(cfg, rec); err != nil {
		return statusForRunnerErr(err)
	}
	if err := ingest(ctx, rec); err != nil {
		return statusForIngestErr(err)
	}
	return RunnerResponse{Status: http.StatusNoContent}
}

func handleRunnerReplay(ctx context.Context, cfg Config, payload json.RawMessage, ingest func(context.Context, TelemetryRecord) error) RunnerResponse {
	var batch RunnerReplayBatch
	if err := decodeStrictJSON(payload, &batch); err != nil {
		return RunnerResponse{Status: http.StatusBadRequest, Error: "invalid replay batch"}
	}
	if len(batch.Records) == 0 {
		return RunnerResponse{Status: http.StatusBadRequest, Error: "empty replay batch"}
	}
	if len(batch.Records) > cfg.RunnerChannel.ReplayMaxRecords || int64(len(payload)) > cfg.RunnerChannel.ReplayMaxBytes {
		return RunnerResponse{Status: http.StatusRequestEntityTooLarge, Error: ErrRunnerPayloadTooLarge.Error()}
	}
	results := make([]RunnerReplayRecordResult, len(batch.Records))
	for i, rec := range batch.Records {
		if err := validateRunnerTelemetryRecord(cfg, rec); err != nil {
			r := statusForRunnerErr(err)
			results[i] = RunnerReplayRecordResult{Status: r.Status, Error: r.Error}
			continue
		}
		if err := ingest(ctx, rec); err != nil {
			r := statusForIngestErr(err)
			results[i] = RunnerReplayRecordResult{Status: r.Status, Error: r.Error}
			continue
		}
		results[i] = RunnerReplayRecordResult{Status: http.StatusNoContent}
	}
	rawResult, err := json.Marshal(RunnerReplayResult{Results: results})
	if err != nil {
		return RunnerResponse{Status: http.StatusInternalServerError, Error: err.Error()}
	}
	return RunnerResponse{Status: http.StatusOK, Result: rawResult}
}

func validateRunnerTelemetryRecord(cfg Config, rec TelemetryRecord) error {
	if strings.TrimSpace(rec.RecordID) == "" || len(rec.Payload) == 0 {
		return fmt.Errorf("runner record requires non-empty record_id and payload")
	}
	if !cfg.RunnerChannel.AllowsTelemetryKind(rec.Kind) {
		return fmt.Errorf("%w: kind %q", ErrRunnerForbidden, rec.Kind)
	}
	if int64(len(rec.Payload)) > cfg.UpdateMaxBodyBytes {
		return ErrRunnerPayloadTooLarge
	}
	return nil
}

func statusForRunnerErr(err error) RunnerResponse {
	switch {
	case errors.Is(err, ErrRunnerForbidden):
		return RunnerResponse{Status: http.StatusForbidden, Error: err.Error()}
	case errors.Is(err, ErrRunnerPayloadTooLarge):
		return RunnerResponse{Status: http.StatusRequestEntityTooLarge, Error: err.Error()}
	default:
		return RunnerResponse{Status: http.StatusBadRequest, Error: err.Error()}
	}
}

func statusForIngestErr(err error) RunnerResponse {
	if errors.Is(err, ErrQuarantine) {
		return RunnerResponse{Status: http.StatusBadRequest, Error: err.Error()}
	}
	return RunnerResponse{Status: http.StatusServiceUnavailable, Error: err.Error()}
}

func parseAndValidateRunnerFD(cfg Config) (int, error) {
	raw := strings.TrimSpace(os.Getenv(cfg.RunnerChannel.FDEnv))
	if raw == "" {
		return 0, fmt.Errorf("%w: %s is unset", ErrInvalidRunnerChannel, cfg.RunnerChannel.FDEnv)
	}
	fd, err := strconv.Atoi(raw)
	if err != nil || fd < 3 {
		return 0, fmt.Errorf("%w: %s=%q", ErrInvalidRunnerChannel, cfg.RunnerChannel.FDEnv, raw)
	}
	if err := validateConnectedUnixSocket(fd, syscall.SOCK_SEQPACKET); err != nil {
		return 0, fmt.Errorf("%w: fd %d: %v", ErrInvalidRunnerChannel, fd, err)
	}
	syscall.CloseOnExec(fd)
	return fd, nil
}

func acquireDirLock(dir string) (*os.File, error) {
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		_ = lock.Close()
		return nil, err
	}
	return lock, nil
}

func releaseDirLock(lock *os.File) {
	if lock == nil {
		return
	}
	_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	_ = lock.Close()
}

func quarantineSpoolFile(dir, path string) {
	quarantineDir := filepath.Join(dir, "quarantine")
	_ = os.MkdirAll(quarantineDir, 0700)
	_ = os.Rename(path, filepath.Join(quarantineDir, filepath.Base(path)))
}
