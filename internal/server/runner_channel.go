package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	RunnerOpTelemetryIngest = "telemetry_ingest"
	RunnerOpTelemetryReplay = "telemetry_replay"
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

	conn, err := openPerRequestStream(fd)
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

func openPerRequestStream(brokerFD int) (net.Conn, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("create per-request socketpair: %w", err)
	}
	clientFile := os.NewFile(uintptr(fds[0]), "harnez-runner-req")
	defer clientFile.Close()
	peerFD := fds[1]
	if err := syscall.Sendmsg(brokerFD, []byte{1}, syscall.UnixRights(peerFD), nil, 0); err != nil {
		_ = syscall.Close(peerFD)
		return nil, fmt.Errorf("%w: sendmsg: %v", ErrRunnerUnavailable, err)
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
// in runner mode, or via the host Unix socket and host spool otherwise.
func DispatchTelemetry(ctx context.Context, cfg Config, record TelemetryRecord) UpdateResult {
	if os.Getenv("TAIA_SESSION_DIR") != "" {
		return SendRunnerTelemetry(ctx, cfg, record)
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
// spilling to the runner's private spool directory only on transient collector
// outages. Missing or invalid broker FDs fail closed.
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
		_ = ReplayRunnerSpool(ctx, cfg)
		return UpdateDelivered
	}
	if errors.Is(err, ErrInvalidRunnerChannel) || errors.Is(err, ErrRunnerPayloadTooLarge) {
		return UpdateDropped
	}
	if err == nil && resp.Status < 500 {
		return UpdateDropped
	}
	spoolDir, err := cfg.TelemetrySpoolDir()
	if err != nil {
		return UpdateDropped
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return UpdateDropped
	}
	if err := AppendTelemetrySpool(spoolDir, raw, cfg.RunnerChannel.SpoolMaxBytes); err != nil {
		return UpdateDropped
	}
	return UpdateSpooled
}

// ReplayRunnerSpool sends a bounded batch of records from the current runner's
// own spool directory over the inherited broker channel.
func ReplayRunnerSpool(ctx context.Context, cfg Config) error {
	if _, err := parseAndValidateRunnerFD(cfg); err != nil {
		return err
	}
	dir, err := cfg.TelemetrySpoolDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	lock, err := acquireDirLock(dir)
	if err != nil {
		return err
	}
	defer releaseDirLock(lock)

	batch, paths, err := collectRunnerReplayBatch(cfg, dir)
	if err != nil || len(batch.Records) == 0 {
		return err
	}
	rawBatch, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	resp, err := SendRunnerRequest(ctx, cfg, RunnerRequest{Op: RunnerOpTelemetryReplay, Payload: rawBatch})
	if err != nil {
		return err
	}
	return applyRunnerReplayResponse(dir, paths, resp)
}

func applyRunnerReplayResponse(dir string, paths []string, resp RunnerResponse) error {
	if resp.Status == http.StatusRequestEntityTooLarge || resp.Status >= 500 {
		return fmt.Errorf("%w: status %d (%s)", ErrRunnerUnavailable, resp.Status, resp.Error)
	}
	if resp.Status < 200 || resp.Status >= 300 {
		for _, p := range paths {
			quarantineSpoolFile(dir, p)
		}
		return fmt.Errorf("runner replay rejected (%d): %s", resp.Status, resp.Error)
	}
	if len(resp.Result) == 0 {
		return removeReplayedPaths(paths)
	}
	var perRecord RunnerReplayResult
	if err := decodeStrictJSON(resp.Result, &perRecord); err != nil || len(perRecord.Results) != len(paths) {
		return fmt.Errorf("invalid replay result metadata")
	}
	var failures []error
	for i, outcome := range perRecord.Results {
		switch {
		case outcome.Status >= 200 && outcome.Status < 300:
			if err := os.Remove(paths[i]); err != nil && !errors.Is(err, fs.ErrNotExist) {
				failures = append(failures, err)
			}
		case outcome.Status >= 400 && outcome.Status < 500 && outcome.Status != http.StatusRequestEntityTooLarge:
			quarantineSpoolFile(dir, paths[i])
		default:
			failures = append(failures, fmt.Errorf("%w: record %d status %d (%s)", ErrRunnerUnavailable, i, outcome.Status, outcome.Error))
		}
	}
	return errors.Join(failures...)
}

func removeReplayedPaths(paths []string) error {
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func collectRunnerReplayBatch(cfg Config, dir string) (RunnerReplayBatch, []string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return RunnerReplayBatch{}, nil, nil
	}
	if err != nil {
		return RunnerReplayBatch{}, nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var batch RunnerReplayBatch
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if len(batch.Records) >= cfg.RunnerChannel.ReplayMaxRecords {
			break
		}
		path := filepath.Join(dir, entry.Name())
		rec, ok := readAndValidateSpoolRecord(cfg, dir, path)
		if !ok {
			continue
		}
		if !fitsReplayWireCap(cfg, batch.Records, rec) {
			if len(batch.Records) == 0 {
				quarantineSpoolFile(dir, path)
				continue
			}
			break
		}
		batch.Records = append(batch.Records, rec)
		paths = append(paths, path)
	}
	return batch, paths, nil
}

func readAndValidateSpoolRecord(cfg Config, dir, path string) (TelemetryRecord, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		quarantineSpoolFile(dir, path)
		return TelemetryRecord{}, false
	}
	var rec TelemetryRecord
	if err := decodeStrictJSON(data, &rec); err != nil || validateRunnerTelemetryRecord(cfg, rec) != nil {
		quarantineSpoolFile(dir, path)
		return TelemetryRecord{}, false
	}
	return rec, true
}

func fitsReplayWireCap(cfg Config, current []TelemetryRecord, next TelemetryRecord) bool {
	probe := RunnerReplayBatch{Records: append(append([]TelemetryRecord(nil), current...), next)}
	encoded, err := json.Marshal(probe)
	if err != nil {
		return false
	}
	return int64(len(encoded)) <= cfg.RunnerChannel.ReplayMaxBytes
}

// ServeRunnerBroker listens on the host end of the AF_UNIX SOCK_SEQPACKET broker
// socketpair, accepts per-request SCM_RIGHTS stream connections concurrently, and
// dispatches validated telemetry operations.
func ServeRunnerBroker(ctx context.Context, hostEnd *os.File, cfg Config, ingest func(context.Context, TelemetryRecord) error) error {
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
	return serveUnixBroker(ctx, seqConn, cfg, ingest)
}

// RelayRunnerBroker bridges the host end of the runner broker socketpair to
// the host collector Unix socket without exposing host-only admin operations.
func RelayRunnerBroker(ctx context.Context, hostEnd *os.File, hostSocket string, cfg Config) error {
	return ServeRunnerBroker(ctx, hostEnd, cfg, hostSocketIngestor(hostSocket, cfg))
}

func serveUnixBroker(ctx context.Context, seqConn *net.UnixConn, cfg Config, ingest func(context.Context, TelemetryRecord) error) error {
	done := make(chan struct{})
	var closeOnce sync.Once
	closeSeq := func() { closeOnce.Do(func() { _ = seqConn.Close() }) }
	defer closeSeq()
	go func() {
		select {
		case <-ctx.Done():
			closeSeq()
		case <-done:
		}
	}()
	defer close(done)

	var wg sync.WaitGroup
	defer wg.Wait()

	buf := make([]byte, 16)
	oob := make([]byte, syscall.CmsgSpace(4*8))
	for {
		_, oobn, _, _, err := seqConn.ReadMsgUnix(buf, oob)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		clientConn, err := extractSCMStreamConn(oob[:oobn])
		if err != nil {
			continue
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			_ = ServeRunnerConn(ctx, c, cfg, ingest)
		}(clientConn)
	}
}

func extractSCMStreamConn(oob []byte) (net.Conn, error) {
	scms, err := syscall.ParseSocketControlMessage(oob)
	if err != nil || len(scms) != 1 {
		return nil, fmt.Errorf("invalid scm control message")
	}
	fds, err := syscall.ParseUnixRights(&scms[0])
	if err != nil {
		return nil, err
	}
	if len(fds) != 1 {
		for _, fd := range fds {
			_ = syscall.Close(fd)
		}
		return nil, fmt.Errorf("expected single fd, got %d", len(fds))
	}
	f := os.NewFile(uintptr(fds[0]), "harnez-broker-client")
	defer f.Close()
	if err := validateConnectedUnixSocket(fds[0], syscall.SOCK_STREAM); err != nil {
		return nil, err
	}
	return net.FileConn(f)
}

func hostSocketIngestor(hostSocket string, cfg Config) func(context.Context, TelemetryRecord) error {
	return func(callCtx context.Context, record TelemetryRecord) error {
		body, err := json.Marshal(record)
		if err != nil {
			return errors.Join(err, ErrQuarantine)
		}
		if _, err := PostTelemetryCommand(callCtx, hostSocket, cfg.TelemetryPath, body, cfg.HTTPClientTimeout); err != nil {
			var statusErr *HTTPStatusError
			if errors.As(err, &statusErr) && statusErr.StatusCode >= 400 && statusErr.StatusCode < 500 {
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
	if conn == nil {
		return fmt.Errorf("serve runner conn: nil connection")
	}
	if ingest == nil {
		return fmt.Errorf("serve runner conn: nil ingest handler")
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
		resp := handleRunnerFrame(ctx, cfg, frame, ingest)
		if err := writeRunnerResponse(conn, cfg.RunnerChannel.MaxFrameBytes, resp); err != nil {
			return err
		}
	}
}

func handleRunnerFrame(ctx context.Context, cfg Config, frame []byte, ingest func(context.Context, TelemetryRecord) error) RunnerResponse {
	var req RunnerRequest
	if err := decodeStrictJSON(frame, &req); err != nil {
		return RunnerResponse{Status: http.StatusBadRequest, Error: "invalid runner request"}
	}
	if strings.TrimSpace(req.Route) != "" {
		return RunnerResponse{Status: http.StatusForbidden, Error: ErrRunnerForbidden.Error()}
	}
	switch req.Op {
	case RunnerOpTelemetryIngest:
		return handleRunnerIngest(ctx, cfg, req.Payload, ingest)
	case RunnerOpTelemetryReplay:
		return handleRunnerReplay(ctx, cfg, req.Payload, ingest)
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
	return fd, nil
}

func validateConnectedUnixSocket(fd, wantType int) error {
	domain, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_DOMAIN)
	if err != nil || domain != syscall.AF_UNIX {
		return fmt.Errorf("not an AF_UNIX socket")
	}
	sockType, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil || sockType != wantType {
		return fmt.Errorf("unexpected socket type")
	}
	if _, err := syscall.Getpeername(fd); err != nil {
		return fmt.Errorf("not connected: %w", err)
	}
	return nil
}

func writeRunnerResponse(w io.Writer, maxBytes int64, resp RunnerResponse) error {
	raw, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return writeChannelFrame(w, raw, maxBytes)
}

func writeChannelFrame(w io.Writer, body []byte, maxBytes int64) error {
	if maxBytes <= 0 || int64(len(body)) > maxBytes {
		return ErrRunnerPayloadTooLarge
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func readChannelFrame(r io.Reader, maxBytes int64) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 || maxBytes <= 0 || int64(n) > maxBytes {
		return nil, ErrRunnerPayloadTooLarge
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func decodeStrictJSON(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON data")
	}
	return nil
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
