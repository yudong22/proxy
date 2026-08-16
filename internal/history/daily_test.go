package history

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestDailyAggregatesPerDay(t *testing.T) {
	h := New(100)
	loc := time.Now().Location()
	today := time.Date(2026, 8, 16, 10, 0, 0, 0, loc)
	yesterday := today.AddDate(0, 0, -1)

	h.Add(RequestRecord{
		Model: "kimi", StartTime: today, Duration: time.Second,
		InputTokens: 100, OutputTokens: 50, Streaming: true, Success: true,
	})
	h.Add(RequestRecord{
		Model: "qwen", StartTime: today, Duration: time.Second,
		InputTokens: 200, OutputTokens: 25, Streaming: false, Success: false,
	})
	h.Add(RequestRecord{
		Model: "glm", StartTime: yesterday, Duration: time.Second,
		InputTokens: 10, OutputTokens: 5, Success: true,
	})

	// Daily(3) covers yesterday, today, and a zero-filled day before that.
	stats := h.Daily(3)
	if len(stats) != 3 {
		t.Fatalf("expected 3 days, got %d", len(stats))
	}
	// stats[0] = day before yesterday (zero-filled), stats[1] = yesterday, stats[2] = today.
	if stats[0].Requests != 0 {
		t.Errorf("expected zero-filled day, got %+v", stats[0])
	}
	if stats[1].Requests != 1 || stats[1].Success != 1 || stats[1].Failed != 0 {
		t.Errorf("yesterday stats mismatch: %+v", stats[1])
	}
	if stats[2].Requests != 2 || stats[2].Success != 1 || stats[2].Failed != 1 {
		t.Errorf("today stats mismatch: %+v", stats[2])
	}
	if stats[2].Streamed != 1 {
		t.Errorf("today streamed mismatch: %+v", stats[2])
	}
	if stats[2].InputTokens != 300 || stats[2].OutputTokens != 75 {
		t.Errorf("today token stats mismatch: %+v", stats[2])
	}
}

func TestDailyPrunesOldDays(t *testing.T) {
	h := New(100)
	loc := time.Now().Location()
	// Add records spread across maxDailyDays+5 distinct days.
	for i := 0; i < maxDailyDays+5; i++ {
		day := time.Date(2026, 8, 1, 12, 0, 0, 0, loc).AddDate(0, 0, i)
		h.Add(RequestRecord{Model: "m", StartTime: day})
	}
	h.mu.RLock()
	n := len(h.days)
	h.mu.RUnlock()
	if n > maxDailyDays {
		t.Fatalf("expected at most %d days cached, got %d", maxDailyDays, n)
	}
}

func TestSinceReturnsNewestFirst(t *testing.T) {
	h := New(100)
	loc := time.Now().Location()
	base := time.Date(2026, 8, 16, 12, 0, 0, 0, loc)
	cutoff := base.Add(-2 * time.Hour)

	h.Add(RequestRecord{Model: "old", StartTime: base.Add(-4 * time.Hour)})
	h.Add(RequestRecord{Model: "mid", StartTime: base.Add(-1 * time.Hour)})
	h.Add(RequestRecord{Model: "new", StartTime: base})

	got := h.Since(cutoff)
	if len(got) != 2 {
		t.Fatalf("expected 2 records since cutoff, got %d: %+v", len(got), got)
	}
	if got[0].Model != "new" || got[1].Model != "mid" {
		t.Errorf("expected newest-first [new mid], got %+v", got)
	}
}

func TestDailyRebuildsOnLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/history.jsonl"

	h := New(100)
	h.SetPersistPath(path)
	loc := time.Now().Location()
	day := time.Date(2026, 8, 16, 9, 0, 0, 0, loc)
	h.Add(RequestRecord{Model: "a", StartTime: day, InputTokens: 5, Success: true})
	h.Add(RequestRecord{Model: "b", StartTime: day, InputTokens: 7, Success: false})

	h2 := New(100)
	if err := h2.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	stats := h2.Daily(1)
	if len(stats) != 1 || stats[0].Requests != 2 || stats[0].InputTokens != 12 {
		t.Fatalf("daily stats not rebuilt from file: %+v", stats)
	}
}

func TestLoadFromFileDoesNotReAppend(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/history.jsonl"

	h := New(100)
	h.SetPersistPath(path)
	now := time.Now()
	h.Add(RequestRecord{Model: "a", StartTime: now, Success: true})
	h.Add(RequestRecord{Model: "b", StartTime: now, Success: false})
	linesBefore := countLines(t, path)

	// Replaying the file into a fresh History must not write the records back
	// (before the fix, every loaded record re-appended + fsynced the file).
	h2 := New(100)
	h2.SetPersistPath(path)
	if err := h2.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if linesAfter := countLines(t, path); linesAfter != linesBefore {
		t.Fatalf("file grew on load: %d → %d lines", linesBefore, linesAfter)
	}
	if got := h2.Len(); got != 2 {
		t.Fatalf("loaded %d records, want 2", got)
	}
}

func TestLoadFromFileCompactsPrunedDays(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/history.jsonl"

	// Seed a file with one record per day across maxDailyDays+5 distinct days.
	// The oldest few days fall outside the per-day aggregation window.
	h := New(100)
	h.SetPersistPath(path)
	base := time.Now().AddDate(0, 0, -(maxDailyDays + 5))
	for i := 0; i < maxDailyDays+5; i++ {
		h.Add(RequestRecord{Model: "m", StartTime: base.AddDate(0, 0, i), Success: true})
	}
	linesBefore := countLines(t, path)

	h2 := New(100)
	h2.SetPersistPath(path)
	if err := h2.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	// The ring still holds all records; only the persisted file is compacted
	// down to the aggregation window (31 of 36 days).
	if got := h2.Len(); got != maxDailyDays+5 {
		t.Fatalf("loaded %d records into ring, want %d", got, maxDailyDays+5)
	}
	if linesAfter := countLines(t, path); linesAfter >= linesBefore {
		t.Fatalf("expected compact to drop old lines, got %d >= %d", linesAfter, linesBefore)
	}
}

func TestLoadFromFileDeduplicatesDuplicatedLines(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/history.jsonl"

	h := New(100)
	h.SetPersistPath(path)
	now := time.Now()
	h.Add(RequestRecord{Model: "a", StartTime: now, Success: true})
	h.Add(RequestRecord{Model: "b", StartTime: now.Add(-time.Hour), Success: true})

	// Simulate the old bug: the whole file was re-appended on every load.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, data...), 0o644); err != nil {
		t.Fatal(err)
	}

	h2 := New(100)
	h2.SetPersistPath(path)
	if err := h2.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if got := h2.Len(); got != 2 {
		t.Fatalf("loaded %d records, want 2 (deduplicated)", got)
	}
	// The file is compacted back down to the two real records.
	if n := countLines(t, path); n != 2 {
		t.Fatalf("file has %d lines after dedup compact, want 2", n)
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return bytes.Count(data, []byte{'\n'})
}
