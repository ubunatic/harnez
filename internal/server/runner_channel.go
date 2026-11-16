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

// SocketPair creates a connected Unix stream socket pair for a host launcher
// and its spawned runner process.
func SocketPair() (hostEnd, runnerEnd *os.File, err error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("create runner socketpair: %w", err)
	}
	return os.NewFile(uintptr(fds[0]), "harnez-broker-host"), os.NewFile(uintptr(fds[1]), "harnez-broker-runner"), nil
}

// SendRunnerRequest sends one framed RPC over the inherited connected broker FD
// configured by cfg.RunnerChannel.FDEnv. Missing or invalid FDs fail closed
// without falling back to the shared host Unix socket.
func SendRunnerRequest(ctx context.Context, cfg Config, req RunnerRequest) (RunnerResponse, error) {
	fd, err := parseAndValidateRunnerFD(cfg)
	if err != nil {
		return RunnerResponse{}, err
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return RunnerResponse{}, fmt.Errorf("%w: lock fd: %v", ErrInvalidRunnerChannel, err)
	}
	defer syscall.Flock(fd, syscall.LOCK_UN) //nolint:errcheck // best-effort unlock

	conn, err := dupFDAsConn(fd)
	if err != nil {
		return RunnerResponse{}, fmt.Errorf("%w: %v", ErrInvalidRunnerChannel, err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else if cfg.HTTPClientTimeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(cfg.HTTPClientTimeout))
	}

	raw, err := json.Marshal(req)
	if err != nil {
		return RunnerResponse{}, err
	}
	if int64(len(raw)) > cfg.RunnerChannel.MaxFrameBytes {
		return RunnerResponse{}, ErrRunnerPayloadTooLarge
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

// RunnerChannelStatus validates that the inherited broker FD env var points to a
// connected Unix stream socket.
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
// spilling to the runner's private spool directory only when the collector is
// temporarily unavailable. Missing or invalid broker FDs fail closed.
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
	if err == nil && resp.Status >= 400 && resp.Status < 500 {
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
	if resp.Status >= 200 && resp.Status < 300 {
		for _, p := range paths {
			if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
		return nil
	}
	if resp.Status >= 400 && resp.Status < 500 {
		for _, p := range paths {
			quarantineSpoolFile(dir, p)
		}
		return fmt.Errorf("runner replay rejected (%d): %s", resp.Status, resp.Error)
	}
	return fmt.Errorf("%w: status %d (%s)", ErrRunnerUnavailable, resp.Status, resp.Error)
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
	var totalBytes int64
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if len(batch.Records) >= cfg.RunnerChannel.ReplayMaxRecords {
			break
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return RunnerReplayBatch{}, nil, err
		}
		var rec TelemetryRecord
		if err := decodeStrictJSON(data, &rec); err != nil || validateRunnerTelemetryRecord(cfg, rec) != nil {
			quarantineSpoolFile(dir, path)
			continue
		}
		encoded, err := json.Marshal(rec)
		if err != nil {
			quarantineSpoolFile(dir, path)
			continue
		}
		recordBytes := int64(len(encoded))
		if len(batch.Records) > 0 && totalBytes+recordBytes > cfg.RunnerChannel.ReplayMaxBytes {
			break
		}
		if recordBytes > cfg.RunnerChannel.ReplayMaxBytes {
			quarantineSpoolFile(dir, path)
			continue
		}
		batch.Records = append(batch.Records, rec)
		paths = append(paths, path)
		totalBytes += recordBytes
	}
	return batch, paths, nil
}

// ServeRunnerConn handles framed runner telemetry RPCs on a host broker end of
// a socketpair, enforcing allowlisted operations, payload caps, and strict JSON.
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
		if errors.Is(err, io.EOF) {
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

// RelayRunnerConn bridges a runner's inherited socketpair end on the host to the
// host collector's Unix socket, permitting only validated telemetry writes and
// never forwarding admin routes such as /shutdown or merge_legacy.
func RelayRunnerConn(ctx context.Context, conn net.Conn, hostSocket string, cfg Config) error {
	return ServeRunnerConn(ctx, conn, cfg, func(callCtx context.Context, record TelemetryRecord) error {
		body, err := json.Marshal(record)
		if err != nil {
			return errors.Join(err, ErrQuarantine)
		}
		if _, err := PostTelemetryCommand(callCtx, hostSocket, cfg.TelemetryPath, body, cfg.HTTPClientTimeout); err != nil {
			if strings.Contains(err.Error(), " 400 ") || strings.Contains(err.Error(), "Bad Request") {
				return errors.Join(err, ErrQuarantine)
			}
			return fmt.Errorf("%w: %v", ErrRunnerUnavailable, err)
		}
		return nil
	})
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
		var rec TelemetryRecord
		if err := decodeStrictJSON(req.Payload, &rec); err != nil {
			return RunnerResponse{Status: http.StatusBadRequest, Error: "invalid telemetry record"}
		}
		if err := validateRunnerTelemetryRecord(cfg, rec); err != nil {
			return statusForRunnerErr(err)
		}
		if err := ingest(ctx, rec); err != nil {
			return statusForIngestErr(err)
		}
		return RunnerResponse{Status: http.StatusNoContent}
	case RunnerOpTelemetryReplay:
		var batch RunnerReplayBatch
		if err := decodeStrictJSON(req.Payload, &batch); err != nil {
			return RunnerResponse{Status: http.StatusBadRequest, Error: "invalid replay batch"}
		}
		if len(batch.Records) == 0 {
			return RunnerResponse{Status: http.StatusBadRequest, Error: "empty replay batch"}
		}
		if len(batch.Records) > cfg.RunnerChannel.ReplayMaxRecords || int64(len(req.Payload)) > cfg.RunnerChannel.ReplayMaxBytes {
			return RunnerResponse{Status: http.StatusRequestEntityTooLarge, Error: ErrRunnerPayloadTooLarge.Error()}
		}
		for _, rec := range batch.Records {
			if err := validateRunnerTelemetryRecord(cfg, rec); err != nil {
				return statusForRunnerErr(err)
			}
		}
		for _, rec := range batch.Records {
			if err := ingest(ctx, rec); err != nil {
				return statusForIngestErr(err)
			}
		}
		return RunnerResponse{Status: http.StatusNoContent}
	default:
		return RunnerResponse{Status: http.StatusForbidden, Error: ErrRunnerForbidden.Error()}
	}
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
	sockType, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil || sockType != syscall.SOCK_STREAM {
		return 0, fmt.Errorf("%w: fd %d is not a stream socket", ErrInvalidRunnerChannel, fd)
	}
	if _, err := syscall.Getpeername(fd); err != nil {
		return 0, fmt.Errorf("%w: fd %d is not connected", ErrInvalidRunnerChannel, fd)
	}
	return fd, nil
}

func dupFDAsConn(fd int) (net.Conn, error) {
	syscall.ForkLock.RLock()
	dupFD, err := syscall.Dup(fd)
	if err == nil {
		syscall.CloseOnExec(dupFD)
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(dupFD), "harnez-broker-dup")
	defer f.Close()
	return net.FileConn(f)
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
