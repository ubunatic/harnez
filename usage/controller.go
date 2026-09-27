package usage

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProtocolVersion is the version of the local controller protocol.
const ProtocolVersion = 1

// DefaultMinRefreshInterval is the smallest interval between live requests for
// one provider. Refresh requests never bypass this protection.
const DefaultMinRefreshInterval = 30 * time.Second

// ErrControllerUnsupported reports that this platform has no local controller
// transport. File-only reads remain available on every platform.
var ErrControllerUnsupported = errors.New("usage: controller is unsupported on this platform")

// ErrRefreshUnavailable reports that no controller can perform a refresh.
var ErrRefreshUnavailable = errors.New("usage: refresh is unavailable without a controller collector")

// Options configures a Client. Open never starts a controller unless
// StartIfAbsent is set. RuntimeDir is the directory containing user-only
// controller files; when empty it follows XDG_RUNTIME_DIR with a ~/.harnez/run
// fallback. StateDir defaults to StateDir("").
type Options struct {
	StateDir           string
	RuntimeDir         string
	StartIfAbsent      bool
	IdleTimeout        time.Duration
	Collector          Collector
	MinRefreshInterval time.Duration
	// OnIdleShutdown is called by an in-process controller's owner after it
	// shuts down because it has been idle for IdleTimeout. It is never called
	// for a controller this client merely joined.
	OnIdleShutdown func(ControllerInfo)
}

// ControllerInfoVersion is the version of the controller-discovery contract.
// Fields may be added compatibly within this version.
const ControllerInfoVersion = 1

// ControllerState describes this client's relationship to a controller.
type ControllerState string

const (
	ControllerUnavailable ControllerState = "unavailable"
	ControllerStarted     ControllerState = "started"
	ControllerJoined      ControllerState = "joined"
)

// ControllerInfo is versioned lifecycle information for the local controller.
// PID is the controller process ID, SocketPath is its Unix-domain socket, and
// ProtocolVersion is the local IPC protocol it serves.
type ControllerInfo struct {
	SchemaVersion   int             `json:"schema_version"`
	State           ControllerState `json:"state"`
	PID             int             `json:"pid,omitempty"`
	SocketPath      string          `json:"socket_path,omitempty"`
	ProtocolVersion int             `json:"protocol_version"`
	IdleTimeout     time.Duration   `json:"idle_timeout,omitempty"`
}

// Collector performs one provider's live collection. The controller, rather
// than the collector, serializes and persists returned snapshots.
type Collector interface {
	CollectUsage(context.Context, ProviderID) (Snapshot, error)
}

// CollectorFunc adapts a function to Collector.
type CollectorFunc func(context.Context, ProviderID) (Snapshot, error)

// CollectUsage implements Collector.
func (f CollectorFunc) CollectUsage(ctx context.Context, provider ProviderID) (Snapshot, error) {
	return f(ctx, provider)
}

// ProviderError carries safe retry metadata for a collector failure.
type ProviderError struct {
	Category   ErrorCategory
	RetryAfter time.Time
}

// Error implements error without exposing provider diagnostics.
func (e ProviderError) Error() string { return "usage: provider collection failed" }

// Client reads persisted snapshots and, when available, uses a local controller.
// Close only releases this client's connection; it never stops a controller it
// did not start.
type Client interface {
	ControllerInfo() ControllerInfo
	Snapshot(context.Context, ProviderID) (*Snapshot, error)
	Subscribe(context.Context, ProviderID) (<-chan SnapshotEvent, error)
	Refresh(context.Context, ...ProviderID) ([]Snapshot, error)
	Close() error
}

// SnapshotEvent is emitted when a controller publishes a changed snapshot.
type SnapshotEvent struct {
	Snapshot      Snapshot `json:"snapshot"`
	RefreshingPID int      `json:"refreshing_pid,omitempty"`
}

type client struct {
	opts           Options
	controllerInfo ControllerInfo
}

// Open opens a file-reading client and attaches to an already-running local
// controller when possible. With StartIfAbsent it starts one foreground
// in-process controller after winning the runtime lock; it never detaches a
// process. The started controller stops after 60 seconds without clients unless
// IdleTimeout supplies another positive duration.
func Open(opts Options) (Client, error) {
	if opts.StateDir == "" {
		opts.StateDir = StateDir("")
	}
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = 60 * time.Second
	}
	if opts.MinRefreshInterval == 0 {
		opts.MinRefreshInterval = DefaultMinRefreshInterval
	}
	if !opts.StartIfAbsent {
		info, err := controllerInfo(context.Background(), opts)
		if err != nil {
			info = unavailableControllerInfo(opts)
		} else {
			info.State = ControllerJoined
		}
		return &client{opts: opts, controllerInfo: info}, nil
	}
	info, err := startIfAbsent(opts)
	if err != nil {
		return nil, err
	}
	return &client{opts: opts, controllerInfo: info}, nil
}

// ControllerInfo reports whether this client started, joined, or could not
// find a controller when it was opened.
func (c *client) ControllerInfo() ControllerInfo { return c.controllerInfo }

func (c *client) Snapshot(ctx context.Context, provider ProviderID) (*Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	response, err := requestController(ctx, c.opts, wireRequest{Version: ProtocolVersion, Operation: "snapshot", Providers: []ProviderID{provider}})
	if err == nil {
		return response.snapshot(provider), nil
	}
	return ReadSnapshot(c.opts.StateDir, provider)
}

func (c *client) Subscribe(ctx context.Context, provider ProviderID) (<-chan SnapshotEvent, error) {
	return subscribeController(ctx, c.opts, provider)
}

func (c *client) Refresh(ctx context.Context, providers ...ProviderID) ([]Snapshot, error) {
	response, err := requestController(ctx, c.opts, wireRequest{Version: ProtocolVersion, Operation: "refresh", Providers: providers, ClientPID: os.Getpid()})
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, errors.New(response.Error)
	}
	return response.Snapshots, nil
}

func (c *client) Close() error { return nil }

type wireRequest struct {
	Version   int          `json:"version"`
	Operation string       `json:"operation"`
	Providers []ProviderID `json:"providers,omitempty"`
	ClientPID int          `json:"client_pid,omitempty"`
}

type wireResponse struct {
	Version    int            `json:"version"`
	Snapshots  []Snapshot     `json:"snapshots,omitempty"`
	Error      string         `json:"error,omitempty"`
	Controller ControllerInfo `json:"controller,omitempty"`
}

func (r wireResponse) snapshot(provider ProviderID) *Snapshot {
	for i := range r.Snapshots {
		if r.Snapshots[i].ProviderID == provider {
			snapshot := r.Snapshots[i]
			return &snapshot
		}
	}
	return nil
}

type controller struct {
	opts           Options
	listener       net.Listener
	lock           *os.File
	mu             sync.Mutex
	clients        int
	waiters        map[uint64]chan SnapshotEvent
	nextID         uint64
	closed         chan struct{}
	close          sync.Once
	lastRefresh    map[ProviderID]time.Time
	inFlight       map[ProviderID]*refreshFlight
	onIdleShutdown func(ControllerInfo)
}

type refreshFlight struct {
	done     chan struct{}
	snapshot Snapshot
}

func (c *controller) serve() {
	for {
		conn, err := c.listener.Accept()
		if err != nil {
			select {
			case <-c.closed:
				return
			default:
				continue
			}
		}
		go c.handle(conn)
	}
}

func (c *controller) shutdownWhenIdle() {
	timer := time.NewTimer(c.opts.IdleTimeout)
	defer timer.Stop()
	for {
		select {
		case <-c.closed:
			return
		case <-timer.C:
			c.mu.Lock()
			active := c.clients
			c.mu.Unlock()
			if active == 0 {
				info := c.info(ControllerStarted)
				c.shutdown()
				if c.onIdleShutdown != nil {
					c.onIdleShutdown(info)
				}
				return
			}
			timer.Reset(c.opts.IdleTimeout)
		}
	}
}

func (c *controller) shutdown() {
	c.close.Do(func() {
		close(c.closed)
		c.mu.Lock()
		for id, waiter := range c.waiters {
			delete(c.waiters, id)
			close(waiter)
		}
		c.mu.Unlock()
		closeController(c)
	})
}

func (c *controller) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	decoder := json.NewDecoder(bufio.NewReader(ioLimitReader(conn)))
	var request wireRequest
	if err := decoder.Decode(&request); err != nil {
		return
	}
	if request.Version != ProtocolVersion {
		c.reply(conn, wireResponse{Version: ProtocolVersion, Error: fmt.Sprintf("usage: incompatible controller protocol %d", request.Version)})
		return
	}
	switch request.Operation {
	case "info":
		c.reply(conn, wireResponse{Version: ProtocolVersion, Controller: c.info(ControllerJoined)})
	case "snapshot":
		c.reply(conn, c.read(request.Providers))
	case "refresh":
		c.reply(conn, wireResponse{Version: ProtocolVersion, Snapshots: c.refresh(request.Providers, request.ClientPID)})
	case "subscribe":
		c.subscribe(conn, request.Providers)
	default:
		c.reply(conn, wireResponse{Version: ProtocolVersion, Error: "usage: invalid controller operation"})
	}
}

func (c *controller) read(providers []ProviderID) wireResponse {
	response := wireResponse{Version: ProtocolVersion}
	for _, provider := range providers {
		snapshot, err := ReadSnapshot(c.opts.StateDir, provider)
		if err != nil {
			response.Error = err.Error()
			return response
		}
		if snapshot != nil {
			response.Snapshots = append(response.Snapshots, *snapshot)
		}
	}
	return response
}

func (c *controller) reply(conn net.Conn, response wireResponse) {
	_ = json.NewEncoder(conn).Encode(response)
}

func (c *controller) subscribe(conn net.Conn, providers []ProviderID) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	updates := make(chan SnapshotEvent, 1)
	c.waiters[id] = updates
	c.clients++
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.waiters, id)
		c.clients--
		c.mu.Unlock()
	}()
	for _, snapshot := range c.read(providers).Snapshots {
		if !c.writeEvent(conn, SnapshotEvent{Snapshot: snapshot}) {
			return
		}
	}
	for event := range updates {
		if !c.writeEvent(conn, event) {
			return
		}
	}
}

func (c *controller) writeEvent(conn net.Conn, event SnapshotEvent) bool {
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return json.NewEncoder(conn).Encode(event) == nil
}

func (c *controller) publish(snapshot Snapshot, refreshingPID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, waiter := range c.waiters {
		select {
		case waiter <- SnapshotEvent{Snapshot: snapshot, RefreshingPID: refreshingPID}:
		default:
		}
	}
}

func (c *controller) refresh(providers []ProviderID, refreshingPID int) []Snapshot {
	if len(providers) == 0 {
		providers = []ProviderID{ProviderClaude, ProviderAGY, ProviderCodex}
	}
	result := make([]Snapshot, 0, len(providers))
	for _, provider := range providers {
		result = append(result, c.refreshProvider(provider, refreshingPID))
	}
	return result
}

func (c *controller) refreshProvider(provider ProviderID, refreshingPID int) Snapshot {
	if !validProviderID(provider) {
		return c.failedSnapshot(provider, ErrorProviderFetch, time.Time{})
	}
	c.mu.Lock()
	if flight := c.inFlight[provider]; flight != nil {
		c.mu.Unlock()
		<-flight.done
		return flight.snapshot
	}
	if last := c.lastRefresh[provider]; !last.IsZero() && time.Since(last) < c.opts.MinRefreshInterval {
		c.mu.Unlock()
		return c.throttledSnapshot(provider, last.Add(c.opts.MinRefreshInterval))
	}
	flight := &refreshFlight{done: make(chan struct{})}
	c.inFlight[provider] = flight
	c.mu.Unlock()

	snapshot := c.collectAndPersist(provider)
	c.mu.Lock()
	c.lastRefresh[provider] = time.Now()
	flight.snapshot = snapshot
	delete(c.inFlight, provider)
	close(flight.done)
	c.mu.Unlock()
	c.publish(snapshot, refreshingPID)
	return snapshot
}

func (c *controller) info(state ControllerState) ControllerInfo {
	return ControllerInfo{SchemaVersion: ControllerInfoVersion, State: state, PID: os.Getpid(), SocketPath: socketPath(c.opts), ProtocolVersion: ProtocolVersion, IdleTimeout: c.opts.IdleTimeout}
}

func controllerInfo(ctx context.Context, opts Options) (ControllerInfo, error) {
	response, err := requestController(ctx, opts, wireRequest{Version: ProtocolVersion, Operation: "info"})
	if err != nil {
		return ControllerInfo{}, err
	}
	return response.Controller, nil
}

func unavailableControllerInfo(opts Options) ControllerInfo {
	return ControllerInfo{SchemaVersion: ControllerInfoVersion, State: ControllerUnavailable, SocketPath: socketPath(opts), ProtocolVersion: ProtocolVersion, IdleTimeout: opts.IdleTimeout}
}

func (c *controller) collectAndPersist(provider ProviderID) Snapshot {
	if c.opts.Collector == nil {
		return c.failedSnapshot(provider, ErrorProviderFetch, time.Time{})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	snapshot, err := c.opts.Collector.CollectUsage(ctx, provider)
	if err != nil {
		category, retryAfter := providerFailure(err)
		return c.failedSnapshot(provider, category, retryAfter)
	}
	snapshot.SchemaVersion = SnapshotSchemaVersion
	snapshot.ProviderID = provider
	if snapshot.Status != StatusDemo {
		snapshot.Status = StatusLive
	}
	if snapshot.Source == "" {
		snapshot.Source = SourceLive
	}
	snapshot.Error = nil
	if snapshot.FetchedAt.IsZero() {
		snapshot.FetchedAt = time.Now().UTC()
	}
	if snapshot.ObservedAt.IsZero() {
		snapshot.ObservedAt = snapshot.FetchedAt
	}
	if err := writeSnapshot(c.opts.StateDir, snapshot); err != nil {
		return c.failedSnapshot(provider, ErrorProviderFetch, time.Time{})
	}
	return snapshot
}

func providerFailure(err error) (ErrorCategory, time.Time) {
	var providerErr ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Category, providerErr.RetryAfter
	}
	return ErrorProviderFetch, time.Time{}
}

func (c *controller) failedSnapshot(provider ProviderID, category ErrorCategory, retryAfter time.Time) Snapshot {
	snapshot, err := ReadSnapshot(c.opts.StateDir, provider)
	if err == nil && snapshot != nil {
		snapshot.Status = StatusStale
		snapshot.Error = &FetchError{Category: category}
		if !retryAfter.IsZero() {
			snapshot.Error.RetryAfter = &retryAfter
		}
		return *snapshot
	}
	now := time.Now().UTC()
	return Snapshot{SchemaVersion: SnapshotSchemaVersion, ProviderID: provider, FetchedAt: now, Status: StatusError, Error: &FetchError{Category: category}}
}

func (c *controller) throttledSnapshot(provider ProviderID, retryAfter time.Time) Snapshot {
	snapshot, err := ReadSnapshot(c.opts.StateDir, provider)
	if err == nil && snapshot != nil {
		snapshot.Status = StatusThrottled
		snapshot.Error = &FetchError{Category: ErrorThrottled, RetryAfter: &retryAfter}
		return *snapshot
	}
	now := time.Now().UTC()
	return Snapshot{SchemaVersion: SnapshotSchemaVersion, ProviderID: provider, FetchedAt: now, Status: StatusThrottled, Error: &FetchError{Category: ErrorThrottled, RetryAfter: &retryAfter}}
}

func writeSnapshot(stateDir string, snapshot Snapshot) error {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return fmt.Errorf("usage: create state directory: %w", err)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("usage: encode snapshot: %w", err)
	}
	path := filepath.Join(stateDir, string(snapshot.ProviderID)+".json")
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0600); err != nil {
		return fmt.Errorf("usage: write snapshot: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("usage: publish snapshot: %w", err)
	}
	return nil
}

func ioLimitReader(conn net.Conn) *io.LimitedReader { return &io.LimitedReader{R: conn, N: 1 << 20} }
