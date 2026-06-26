// Package history maintains an in-memory ring buffer of recent proxy requests.
package history

import "sync"

const defaultMaxRecords = 1000

// History is a thread-safe ring buffer of RequestRecord entries.
type History struct {
	mu      sync.RWMutex
	records []RequestRecord
	cap     int
}

// New creates a History that retains at most maxRecords entries.
// If maxRecords is 0, the default of 1000 is used.
func New(maxRecords int) *History {
	if maxRecords <= 0 {
		maxRecords = defaultMaxRecords
	}
	return &History{
		records: make([]RequestRecord, 0, maxRecords),
		cap:     maxRecords,
	}
}

// Add appends a record to the history. If the buffer is full, the oldest
// entry is evicted (ring-buffer behaviour).
func (h *History) Add(r RequestRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.records) >= h.cap {
		// shift left, drop oldest
		copy(h.records, h.records[1:])
		h.records[len(h.records)-1] = r
	} else {
		h.records = append(h.records, r)
	}
}

// Last returns up to n most-recent records in newest-first order.
// If n <= 0 all records are returned.
func (h *History) Last(n int) []RequestRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	total := len(h.records)
	if n <= 0 || n > total {
		n = total
	}
	// copy so the caller cannot mutate internal state
	out := make([]RequestRecord, n)
	for i := 0; i < n; i++ {
		out[i] = h.records[total-n+i]
	}
	// reverse to newest-first
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// Len returns the current number of records stored.
func (h *History) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.records)
}
