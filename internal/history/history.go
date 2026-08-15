// Package history maintains an in-memory ring buffer of recent proxy requests.
package history

import (
	"log/slog"
	"sync"
)

const defaultMaxRecords = 1000

// History is a thread-safe ring buffer of RequestRecord entries.
// Uses head/tail indices for O(1) insert instead of O(n) slice shift.
// When persistPath is set, every Add also appends a JSONL line to that file so
// history survives restarts. Persistence never blocks or fails the caller.
type History struct {
	mu          sync.RWMutex
	records     []RequestRecord
	head        int // write position
	count       int // number of records stored
	cap         int // max capacity
	persistPath string
	persistMu   sync.Mutex
}

// New creates a History that retains at most maxRecords entries.
// If maxRecords is 0, the default of 1000 is used.
func New(maxRecords int) *History {
	if maxRecords <= 0 {
		maxRecords = defaultMaxRecords
	}
	return &History{
		records: make([]RequestRecord, maxRecords),
		cap:     maxRecords,
	}
}

// Add appends a record to the history. If the buffer is full, the oldest
// entry is evicted (ring-buffer behaviour). O(1) time complexity.
func (h *History) Add(r RequestRecord) {
	h.mu.Lock()
	h.records[h.head] = r
	h.head = (h.head + 1) % h.cap
	if h.count < h.cap {
		h.count++
	}
	path := h.persistPath
	h.mu.Unlock()

	if path != "" {
		h.persistMu.Lock()
		defer h.persistMu.Unlock()
		if err := appendRecord(path, r); err != nil {
			// Persistence is best-effort: never fail the request path.
			slog.Warn("history persist failed", "err", err)
		}
	}
}

// SetPersistPath enables append-only JSONL persistence to the given file.
// Must be called before any Add if persistence is desired.
func (h *History) SetPersistPath(path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.persistPath = path
}

// LoadFromFile replays records from a JSONL history file into the ring buffer
// in chronological order. A missing file is not an error.
func (h *History) LoadFromFile(path string) error {
	records, err := loadRecords(path)
	if err != nil {
		return err
	}
	for _, r := range records {
		h.Add(r)
	}
	return nil
}

// Last returns up to n most-recent records in newest-first order.
// If n <= 0 all records are returned.
func (h *History) Last(n int) []RequestRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n <= 0 || n > h.count {
		n = h.count
	}
	out := make([]RequestRecord, n)
	// Iterate backwards from head-1 (most recent) to head-n (oldest of the n).
	for i := 0; i < n; i++ {
		idx := (h.head - 1 - i + h.cap) % h.cap
		out[i] = h.records[idx]
	}
	return out
}

// Len returns the current number of records stored.
func (h *History) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.count
}
