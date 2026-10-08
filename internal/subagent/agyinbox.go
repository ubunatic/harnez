package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const defaultAgyInboxMaxMessageBytes = 16384

// InboxMessage is a persisted message waiting for, or reporting, delivery.
type InboxMessage struct {
	ID          string    `json:"id"`
	SenderID    string    `json:"sender_id,omitempty"`
	Text        string    `json:"text"`
	State       string    `json:"state"`
	QueuedAt    time.Time `json:"queued_at"`
	DeliveredAt time.Time `json:"delivered_at,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// Inbox stores queued messages as individual JSON files for restart recovery.
type Inbox struct {
	dir     string
	maxSize int
	mu      sync.Mutex
	queue   []*InboxMessage
	all     map[string]*InboxMessage
	wake    chan struct{}
}

// OpenInbox loads queued messages from dir in enqueue order.
func OpenInbox(dir string) (*Inbox, error) {
	return OpenInboxWithLimit(dir, defaultAgyInboxMaxMessageBytes)
}

// OpenInboxWithLimit loads queued messages and sets the maximum UTF-8 byte size.
func OpenInboxWithLimit(dir string, maxMessageBytes int) (*Inbox, error) {
	if maxMessageBytes < 0 {
		return nil, fmt.Errorf("inbox message byte limit must not be negative")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create inbox directory: %w", err)
	}
	inbox := &Inbox{dir: dir, maxSize: maxMessageBytes, all: make(map[string]*InboxMessage), wake: make(chan struct{}, 1)}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read inbox directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read inbox message %q: %w", entry.Name(), err)
		}
		var msg InboxMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("decode inbox message %q: %w", entry.Name(), err)
		}
		if msg.State == "queued" {
			inbox.queue = append(inbox.queue, &msg)
		}
		inbox.all[msg.ID] = &msg
	}
	sort.SliceStable(inbox.queue, func(i, j int) bool {
		return inbox.queue[i].QueuedAt.Before(inbox.queue[j].QueuedAt)
	})
	if len(inbox.queue) > 0 {
		inbox.signal()
	}
	return inbox, nil
}

// Enqueue persists a message before making it visible to consumers.
func (inbox *Inbox) Enqueue(sender, text string) (*InboxMessage, error) {
	if len([]byte(text)) > inbox.maxSize {
		return nil, fmt.Errorf("inbox message exceeds %d byte limit", inbox.maxSize)
	}
	msg := &InboxMessage{ID: uuid.NewString(), SenderID: sender, Text: text, State: "queued", QueuedAt: time.Now().UTC()}
	if err := inbox.write(msg); err != nil {
		return nil, err
	}
	inbox.mu.Lock()
	inbox.queue = append(inbox.queue, msg)
	inbox.all[msg.ID] = msg
	inbox.mu.Unlock()
	inbox.signal()
	return cloneInboxMessage(msg), nil
}

// Next removes and returns the next queued message, or nil when empty.
func (inbox *Inbox) Next() *InboxMessage {
	inbox.mu.Lock()
	defer inbox.mu.Unlock()
	if len(inbox.queue) == 0 {
		return nil
	}
	msg := inbox.queue[0]
	inbox.queue = inbox.queue[1:]
	return cloneInboxMessage(msg)
}

// Depth reports the number of messages still waiting in memory.
func (inbox *Inbox) Depth() int {
	inbox.mu.Lock()
	defer inbox.mu.Unlock()
	return len(inbox.queue)
}

// Messages returns queued messages in FIFO order.
func (inbox *Inbox) Messages() []InboxMessage {
	inbox.mu.Lock()
	defer inbox.mu.Unlock()
	messages := make([]InboxMessage, 0, len(inbox.queue))
	for _, msg := range inbox.queue {
		messages = append(messages, *cloneInboxMessage(msg))
	}
	return messages
}

// Mark updates a message's persisted delivery state.
func (inbox *Inbox) Mark(id, state, errText string) error {
	if state != "queued" && state != "delivered" && state != "failed" {
		return fmt.Errorf("invalid inbox message state %q", state)
	}
	inbox.mu.Lock()
	defer inbox.mu.Unlock()
	msg := inbox.all[id]
	if msg == nil {
		data, err := os.ReadFile(inbox.messagePath(id))
		if err != nil {
			return fmt.Errorf("read inbox message %q: %w", id, err)
		}
		msg = &InboxMessage{}
		if err := json.Unmarshal(data, msg); err != nil {
			return fmt.Errorf("decode inbox message %q: %w", id, err)
		}
	}
	updated := *msg
	updated.State = state
	updated.Error = errText
	if state == "delivered" {
		updated.DeliveredAt = time.Now().UTC()
	} else {
		updated.DeliveredAt = time.Time{}
	}
	if err := inbox.write(&updated); err != nil {
		return err
	}
	*msg = updated
	inbox.all[id] = msg
	if state == "queued" {
		inbox.queue = append(inbox.queue, msg)
		sort.SliceStable(inbox.queue, func(i, j int) bool { return inbox.queue[i].QueuedAt.Before(inbox.queue[j].QueuedAt) })
		inbox.signal()
	}
	return nil
}

func (inbox *Inbox) write(msg *InboxMessage) error {
	data, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode inbox message: %w", err)
	}
	if err := os.MkdirAll(inbox.dir, 0o700); err != nil {
		return fmt.Errorf("create inbox directory: %w", err)
	}
	tmp, err := os.CreateTemp(inbox.dir, ".message-*.tmp")
	if err != nil {
		return fmt.Errorf("create inbox message temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure inbox message temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write inbox message: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync inbox message: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close inbox message: %w", err)
	}
	if err := os.Rename(tmpName, inbox.messagePath(msg.ID)); err != nil {
		return fmt.Errorf("persist inbox message: %w", err)
	}
	return nil
}

func (inbox *Inbox) messagePath(id string) string {
	return filepath.Join(inbox.dir, id+".json")
}

func (inbox *Inbox) signal() {
	select {
	case inbox.wake <- struct{}{}:
	default:
	}
}

func cloneInboxMessage(msg *InboxMessage) *InboxMessage {
	if msg == nil {
		return nil
	}
	copy := *msg
	return &copy
}
