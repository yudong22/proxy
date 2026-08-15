package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// persistedRecord is the on-disk shape for a RequestRecord. It mirrors the GUI
// wire format (internal/gui/server.go historyEntry) so the JSONL file is stable
// and human-readable. time.Time / time.Duration are flattened to string/int64.
type persistedRecord struct {
	ID                       string `json:"id"`
	Model                    string `json:"model"`
	Provider                 string `json:"provider"`
	Scenario                 string `json:"scenario"`
	StartTime                string `json:"start_time"` // RFC3339
	DurationMs               int64  `json:"duration_ms"`
	InputTokens              int    `json:"input_tokens"`
	OutputTokens             int    `json:"output_tokens"`
	CacheReadInputTokens     int    `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
	Streaming                bool   `json:"streaming"`
	Success                  bool   `json:"success"`
	ErrorMsg                 string `json:"error_msg,omitempty"`
}

func (p persistedRecord) toRequestRecord() RequestRecord {
	return RequestRecord{
		ID:                       p.ID,
		Model:                    p.Model,
		Provider:                 p.Provider,
		Scenario:                 p.Scenario,
		StartTime:                mustParseTime(p.StartTime),
		Duration:                 time.Duration(p.DurationMs) * time.Millisecond,
		InputTokens:              p.InputTokens,
		OutputTokens:             p.OutputTokens,
		CacheReadInputTokens:     p.CacheReadInputTokens,
		CacheCreationInputTokens: p.CacheCreationInputTokens,
		Streaming:                p.Streaming,
		Success:                  p.Success,
		ErrorMsg:                 p.ErrorMsg,
	}
}

func toPersistedRecord(r RequestRecord) persistedRecord {
	return persistedRecord{
		ID:                       r.ID,
		Model:                    r.Model,
		Provider:                 r.Provider,
		Scenario:                 r.Scenario,
		StartTime:                r.StartTime.Format(time.RFC3339),
		DurationMs:               r.Duration.Milliseconds(),
		InputTokens:              r.InputTokens,
		OutputTokens:             r.OutputTokens,
		CacheReadInputTokens:     r.CacheReadInputTokens,
		CacheCreationInputTokens: r.CacheCreationInputTokens,
		Streaming:                r.Streaming,
		Success:                  r.Success,
		ErrorMsg:                 r.ErrorMsg,
	}
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// appendRecord appends one request record as a JSONL line to the given path,
// creating the parent directory and file if needed. Pattern mirrors
// internal/debug/storage.go's append-open idiom.
func appendRecord(path string, r RequestRecord) error {
	data, err := json.Marshal(toPersistedRecord(r))
	if err != nil {
		return fmt.Errorf("history marshal: %w", err)
	}
	data = append(data, '\n')

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("history mkdir: %w", err)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("history open: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("history write: %w", err)
	}
	return f.Sync()
}

// loadRecords reads a JSONL history file and returns the records oldest-first,
// so they can be replayed back into the ring buffer in chronological order.
// A missing file yields an empty slice (not an error).
func loadRecords(path string) ([]RequestRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("history open: %w", err)
	}
	defer f.Close()

	var out []RequestRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var p persistedRecord
		if err := json.Unmarshal(line, &p); err != nil {
			// Skip malformed lines rather than failing the whole file.
			continue
		}
		out = append(out, p.toRequestRecord())
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("history scan: %w", err)
	}
	return out, nil
}
