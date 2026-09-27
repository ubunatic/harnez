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
	"sync"
	"time"
)

// ProtocolVersion is the version of the local controller protocol.
const ProtocolVersion = 1

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
	StateDir      string
	RuntimeDir    string
	StartIfAbsent bool
	IdleTimeout   time.Duration
}

// Client reads persisted snapshots and, when available, uses a local controller.
// Close only releases this client's connection; it never stops a controller it
// did not start.
type Client interface {
	Snapshot(context.Context, ProviderID) (*Snapshot, error)
	Subscribe(context.Context, ProviderID) (<-chan SnapshotEvent, error)
	Refresh(context.Context, ...ProviderID) ([]Snapshot, error)
	Close() error
}

// SnapshotEvent is emitted when a controller publishes a changed snapshot.
type SnapshotEvent struct {
	Snapshot Snapshot `json:"snapshot"`
}

type client struct {
	opts Options
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
	if !opts.StartIfAbsent {
		return &client{opts: opts}, nil
	}
	if err := startIfAbsent(opts); err != nil {
		return nil, err
	}
	return &client{opts: opts}, nil
}

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
	response, err := requestController(ctx, c.opts, wireRequest{Version: ProtocolVersion, Operation: "refresh", Providers: providers})
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
}

type wireResponse struct {
	Version   int        `json:"version"`
	Snapshots []Snapshot `json:"snapshots,omitempty"`
	Error     string     `json:"error,omitempty"`
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
	opts     Options
	listener net.Listener
	lock     *os.File
	mu       sync.Mutex
	clients  int
	waiters  map[uint64]chan SnapshotEvent
	nextID   uint64
	closed   chan struct{}
	close    sync.Once
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
	case "snapshot":
		c.reply(conn, c.read(request.Providers))
	case "refresh":
		c.reply(conn, wireResponse{Version: ProtocolVersion, Error: ErrRefreshUnavailable.Error()})
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

func (c *controller) publish(snapshot Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, waiter := range c.waiters {
		select {
		case waiter <- SnapshotEvent{Snapshot: snapshot}:
		default:
		}
	}
}

func ioLimitReader(conn net.Conn) *io.LimitedReader { return &io.LimitedReader{R: conn, N: 1 << 20} }
