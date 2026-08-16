package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/routatic/proxy/internal/history"
)

func TestHandleMetrics_DerivesTodayFromHistory(t *testing.T) {
	s := newTestServer()
	h := history.New(100)
	now := time.Now()
	h.Add(history.RequestRecord{
		Model: "kimi", StartTime: now, Duration: time.Second,
		InputTokens: 100, OutputTokens: 50,
		CacheReadInputTokens: 30, CacheCreationInputTokens: 10,
		Streaming: true, Success: true,
	})
	h.Add(history.RequestRecord{
		Model: "qwen", StartTime: now, Duration: time.Second,
		InputTokens: 200, OutputTokens: 25, Success: false,
	})
	// Old record — must not count toward today's cards.
	h.Add(history.RequestRecord{
		Model: "glm", StartTime: now.Add(-48 * time.Hour), Duration: time.Second,
		InputTokens: 999, Success: true,
	})
	s.hist = h

	resp := decodeMetricsResponse(t, s)
	if resp.RequestsReceived != 2 {
		t.Errorf("RequestsReceived = %d, want 2", resp.RequestsReceived)
	}
	if resp.RequestsSuccess != 1 || resp.RequestsFailed != 1 {
		t.Errorf("success/failed = %d/%d, want 1/1", resp.RequestsSuccess, resp.RequestsFailed)
	}
	// Tokens: input(100) + output(50) + cache_read(30) + cache_creation(10) + input(200) + output(25) = 415
	wantTokens := int64(100 + 50 + 30 + 10 + 200 + 25)
	if resp.TokensToday != wantTokens {
		t.Errorf("TokensToday = %d, want %d", resp.TokensToday, wantTokens)
	}
	if resp.ModelCounts["kimi"] != 1 || resp.ModelCounts["qwen"] != 1 || resp.ModelCounts["glm"] != 0 {
		t.Errorf("model counts mismatch: %+v", resp.ModelCounts)
	}
}

func TestHandleDailyStats_ReturnsCachedDays(t *testing.T) {
	s := newTestServer()
	h := history.New(100)
	now := time.Now()
	h.Add(history.RequestRecord{Model: "a", StartTime: now, InputTokens: 5, Success: true})
	h.Add(history.RequestRecord{Model: "b", StartTime: now, InputTokens: 7, Success: false})
	s.hist = h

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/daily?days=3", nil)
	s.handleDailyStats(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", rec.Code, rec.Body.String())
	}
	var resp dailyStatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v; body = %q", err, rec.Body.String())
	}
	if len(resp.Days) != 3 {
		t.Fatalf("len(days) = %d, want 3", len(resp.Days))
	}
	last := resp.Days[len(resp.Days)-1]
	if last.Requests != 2 || last.InputTokens != 12 {
		t.Errorf("today day stat = %+v, want requests=2 input=12", last)
	}
}

func TestHandleDailyStats_NilHistory_ReturnsEmpty(t *testing.T) {
	s := newTestServer() // hist == nil
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/daily", nil)
	s.handleDailyStats(rec, req)
	var resp dailyStatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Days) != 0 {
		t.Errorf("len(days) = %d, want 0", len(resp.Days))
	}
}

func TestHandleHistory_ReturnsAllRecords(t *testing.T) {
	s := newTestServer()
	h := history.New(1000)
	now := time.Now()
	// More than the old 200-record cap.
	for i := 0; i < 250; i++ {
		h.Add(history.RequestRecord{Model: "m", StartTime: now, Success: true})
	}
	s.hist = h

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	s.handleHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var out []historyEntry
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 250 {
		t.Errorf("len(history) = %d, want 250 (all records, not capped at 200)", len(out))
	}
}
