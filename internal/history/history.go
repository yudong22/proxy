// Package history maintains an in-memory ring buffer of recent proxy requests.
package history

import (
	"log/slog"
	"sort"
	"sync"
	"time"
)

const (
	defaultMaxRecords = 1000
	// maxDailyDays bounds the per-day aggregation map so it cannot grow
	// unboundedly even if the ring buffer keeps evicting old records.
	maxDailyDays = 31
)

// DayStat aggregates request counts and token usage for a single calendar day
// (local time). Updated incrementally on every Add so chart reads are O(n)
// over days, not over records.
type DayStat struct {
	Date                string `json:"date"` // "2006-01-02" in local time
	Requests            int    `json:"requests"`
	Success             int    `json:"success"`
	Failed              int    `json:"failed"`
	Streamed            int    `json:"streamed"`
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	CacheReadTokens     int64  `json:"cache_read_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
}

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
	days        map[string]*DayStat // per-day cached aggregation (local dates)
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
		days:    make(map[string]*DayStat),
	}
}

// Add appends a record to the history and persists it when a persist path is
// configured. If the buffer is full, the oldest entry is evicted (ring-buffer
// behaviour). O(1) time complexity (persistence excluded).
func (h *History) Add(r RequestRecord) {
	h.insert(r)

	path := h.persistPath
	if path == "" {
		return
	}
	h.persistMu.Lock()
	defer h.persistMu.Unlock()
	if err := appendRecord(path, r); err != nil {
		// Persistence is best-effort: never fail the request path.
		slog.Warn("history persist failed", "err", err)
	}
}

// insert adds r to the in-memory ring buffer and per-day aggregation without
// touching disk. It is the shared core of Add (which persists afterwards) and
// LoadFromFile (which must never re-persist records that are already on disk —
// doing so would rewrite and fsync the whole file on every startup).
func (h *History) insert(r RequestRecord) {
	h.mu.Lock()
	h.records[h.head] = r
	h.head = (h.head + 1) % h.cap
	if h.count < h.cap {
		h.count++
	}
	h.accumulateDay(r)
	h.mu.Unlock()
}

// accumulateDay updates the per-day cached aggregation for r's local date.
// Must be called with h.mu held.
func (h *History) accumulateDay(r RequestRecord) {
	key := r.StartTime.Format("2006-01-02")
	d := h.days[key]
	if d == nil {
		d = &DayStat{Date: key}
		h.days[key] = d
		// Prune: keep only the most recent maxDailyDays dates.
		if len(h.days) > maxDailyDays {
			dates := make([]string, 0, len(h.days))
			for k := range h.days {
				dates = append(dates, k)
			}
			sort.Strings(dates)
			for _, old := range dates[:len(dates)-maxDailyDays] {
				delete(h.days, old)
			}
		}
	}
	d.Requests++
	if r.Success {
		d.Success++
	} else {
		d.Failed++
	}
	if r.Streaming {
		d.Streamed++
	}
	d.InputTokens += int64(r.InputTokens)
	d.OutputTokens += int64(r.OutputTokens)
	d.CacheReadTokens += int64(r.CacheReadInputTokens)
	d.CacheCreationTokens += int64(r.CacheCreationInputTokens)
}

// SetPersistPath enables append-only JSONL persistence to the given file.
// Must be called before any Add if persistence is desired.
func (h *History) SetPersistPath(path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.persistPath = path
}

// LoadFromFile replays records from a JSONL history file into the ring buffer
// and per-day aggregation in chronological order. It never re-persists: the
// records are already on disk, and re-writing them would fsync the whole file
// once per line (153K F_FULLFSYNCs on a large history). A missing file is not
// an error. After loading, the file is compacted to the aggregation window so
// it cannot grow unboundedly across restarts.
func (h *History) LoadFromFile(path string) error {
	records, rawLines, err := loadRecords(path)
	if err != nil {
		return err
	}
	for _, r := range records {
		h.insert(r)
	}
	h.compact(path, records, rawLines)
	return nil
}

// compact rewrites the persisted file keeping only records whose date still
// contributes to the per-day aggregation (h.days), dropping everything older
// than the aggregation window. rawLines is the on-disk line count before
// deduplication, so the rewrite also triggers when duplicate lines were
// collapsed. Best-effort: failures are logged, never propagated — the
// in-memory state is already fully loaded.
func (h *History) compact(path string, records []RequestRecord, rawLines int) {
	h.mu.RLock()
	keep := make(map[string]bool, len(h.days))
	for k := range h.days {
		keep[k] = true
	}
	h.mu.RUnlock()

	retained := records[:0] // filter in place, oldest-first order preserved
	for _, r := range records {
		if keep[r.StartTime.Format("2006-01-02")] {
			retained = append(retained, r)
		}
	}
	if len(retained) == rawLines {
		return // nothing to drop, no rewrite needed
	}
	if err := rewriteRecords(path, retained); err != nil {
		slog.Warn("history compact failed", "err", err)
	}
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

// Since returns up to all records with StartTime >= t, in newest-first order.
// Used to derive "today" counts that exactly match what the History page shows.
func (h *History) Since(t time.Time) []RequestRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]RequestRecord, 0, h.count)
	for i := 0; i < h.count; i++ {
		idx := (h.head - 1 - i + h.cap) % h.cap
		if !h.records[idx].StartTime.Before(t) {
			out = append(out, h.records[idx])
		}
	}
	return out
}

// Daily returns per-day aggregates for the last n days (including today),
// oldest-first, zero-filled for days with no activity. Reads the cached
// per-day map so it is O(n) regardless of how many records each day holds.
func (h *History) Daily(n int) []DayStat {
	if n <= 0 {
		n = 14
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	out := make([]DayStat, 0, n)
	for i := n - 1; i >= 0; i-- {
		day := today.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		if d := h.days[key]; d != nil {
			out = append(out, *d)
		} else {
			out = append(out, DayStat{Date: key})
		}
	}
	return out
}
