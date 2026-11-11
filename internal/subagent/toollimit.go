package subagent

// ToolCallLimiter enforces a maximum number of tool calls per turn.
type ToolCallLimiter struct {
	limit     int
	attempted int
	seenIDs   map[string]bool
}

// NewToolCallLimiter creates a new limiter with the given limit. A limit of 0 means off.
func NewToolCallLimiter(limit int) *ToolCallLimiter {
	return &ToolCallLimiter{
		limit:   limit,
		seenIDs: make(map[string]bool),
	}
}

// Observe records a tool call event and returns true if the call should be blocked.
func (l *ToolCallLimiter) Observe(e Event) bool {
	if !e.ToolCall {
		return false
	}
	if e.CallID != "" {
		if l.seenIDs[e.CallID] {
			return false // Already counted
		}
		l.seenIDs[e.CallID] = true
	}
	l.attempted++
	if l.limit > 0 && l.attempted > l.limit {
		return true
	}
	return false
}

// Attempted returns the total number of unique tool calls observed.
func (l *ToolCallLimiter) Attempted() int {
	return l.attempted
}

// Reached returns true if the limit has been reached and a tool call was blocked.
func (l *ToolCallLimiter) Reached() bool {
	return l.limit > 0 && l.attempted > l.limit
}

// Limit returns the configured limit.
func (l *ToolCallLimiter) Limit() int {
	return l.limit
}
