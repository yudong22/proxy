package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	rec := RequestRecord{
		ID:                       "req-1",
		Model:                    "deepseek-v4-flash",
		Provider:                 "opencode-go",
		Scenario:                 "default",
		StartTime:                time.Date(2026, 8, 15, 18, 13, 25, 0, time.UTC),
		Duration:                 5951 * time.Millisecond,
		InputTokens:              30727,
		OutputTokens:             536,
		CacheReadInputTokens:     33024,
		CacheCreationInputTokens: 2518,
		Streaming:                false,
		Success:                  true,
	}

	if err := appendRecord(path, rec); err != nil {
		t.Fatalf("appendRecord: %v", err)
	}

	loaded, err := loadRecords(path)
	if err != nil {
		t.Fatalf("loadRecords: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 record, got %d", len(loaded))
	}

	got := loaded[0]
	if got.ID != rec.ID || got.Model != rec.Model || got.Provider != rec.Provider {
		t.Errorf("identity fields mismatch: got %+v want %+v", got, rec)
	}
	if !got.StartTime.Equal(rec.StartTime) {
		t.Errorf("StartTime mismatch: got %v want %v", got.StartTime, rec.StartTime)
	}
	if got.Duration != rec.Duration {
		t.Errorf("Duration mismatch: got %v want %v", got.Duration, rec.Duration)
	}
	if got.CacheReadInputTokens != rec.CacheReadInputTokens || got.CacheCreationInputTokens != rec.CacheCreationInputTokens {
		t.Errorf("cache fields mismatch: got %+v want %+v", got, rec)
	}
	if got.Streaming != rec.Streaming || got.Success != rec.Success {
		t.Errorf("bool fields mismatch: got %+v want %+v", got, rec)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	records, err := loadRecords(filepath.Join(dir, "nope.jsonl"))
	if err != nil {
		t.Fatalf("loadRecords on missing file: %v", err)
	}
	if records != nil && len(records) != 0 {
		t.Fatalf("expected empty, got %d", len(records))
	}
}

func TestLoadSkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	content := "not-json\n{\"model\":\"kimi-k2.6\"}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	records, err := loadRecords(path)
	if err != nil {
		t.Fatalf("loadRecords: %v", err)
	}
	if len(records) != 1 || records[0].Model != "kimi-k2.6" {
		t.Fatalf("expected 1 valid record, got %+v", records)
	}
}

func TestHistoryPersistsOnAddAndLoads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	h := New(100)
	h.SetPersistPath(path)
	h.Add(RequestRecord{Model: "a", StartTime: time.Now()})
	h.Add(RequestRecord{Model: "b", StartTime: time.Now()})

	// Fresh instance loads from the same file.
	h2 := New(100)
	if err := h2.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if h2.Len() != 2 {
		t.Fatalf("expected 2 records after load, got %d", h2.Len())
	}
	all := h2.Last(2)
	if all[0].Model != "b" || all[1].Model != "a" {
		t.Fatalf("order mismatch: %+v", all)
	}
}
