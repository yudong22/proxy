package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer builds a Server with no real dependencies. Metrics and
// History are nil by default — individual tests can swap them in.
func newTestServer() *Server {
	return &Server{
		// hist: nil, met: nil
	}
}

// decodeMetricsResponse runs the request through handleMetrics and
// returns the decoded response body.
func decodeMetricsResponse(t *testing.T, s *Server) metricsResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	s.handleMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp metricsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v; body = %q", err, rec.Body.String())
	}
	return resp
}

func TestHandleMetrics_NilMetrics_ModelCountsIsEmptyMap(t *testing.T) {
	// When the metrics dependency is missing, the response must still
	// contain a non-nil JSON object for "model_counts" (i.e. `{}`), not
	// `null`. The frontend iterates over it directly.
	s := newTestServer() // s.met == nil

	// We also need to capture the raw bytes to check the JSON token.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	s.handleMetrics(rec, req)
	body := rec.Body.String()

	if !strings.Contains(body, `"model_counts":{`) {
		t.Errorf("expected `\"model_counts\":{}` in body, got %q", body)
	}
	if strings.Contains(body, `"model_counts":null`) {
		t.Errorf("model_counts must not serialise as null; body = %q", body)
	}

	resp := decodeMetricsResponse(t, s)
	if resp.ModelCounts == nil {
		t.Error("ModelCounts is nil; expected empty map")
	}
	if len(resp.ModelCounts) != 0 {
		t.Errorf("ModelCounts length = %d, want 0", len(resp.ModelCounts))
	}
}

func TestHandleMetrics_DefaultsToZeroCounters(t *testing.T) {
	s := newTestServer()
	resp := decodeMetricsResponse(t, s)

	if resp.RequestsReceived != 0 {
		t.Errorf("RequestsReceived = %d, want 0", resp.RequestsReceived)
	}
	if resp.RequestsStreamed != 0 {
		t.Errorf("RequestsStreamed = %d, want 0", resp.RequestsStreamed)
	}
	if resp.RequestsSuccess != 0 {
		t.Errorf("RequestsSuccess = %d, want 0", resp.RequestsSuccess)
	}
	if resp.RequestsFailed != 0 {
		t.Errorf("RequestsFailed = %d, want 0", resp.RequestsFailed)
	}
}

func TestHandleMetrics_RunningAndConnectedFlags(t *testing.T) {
	s := newTestServer()
	s.SetProxyRunning(true)
	s.SetConnectedToExisting(true)

	resp := decodeMetricsResponse(t, s)
	if !resp.ProxyRunning {
		t.Error("ProxyRunning = false, want true")
	}
	if !resp.ConnectedExisting {
		t.Error("ConnectedExisting = false, want true")
	}

	s.SetProxyRunning(false)
	s.SetConnectedToExisting(false)
	resp = decodeMetricsResponse(t, s)
	if resp.ProxyRunning {
		t.Error("ProxyRunning = true, want false after SetProxyRunning(false)")
	}
	if resp.ConnectedExisting {
		t.Error("ConnectedExisting = true, want false after SetConnectedToExisting(false)")
	}
}

func TestHandleMetrics_ConnectedWithoutRunningIsPossible(t *testing.T) {
	// The atomic booleans are independent — ConnectedExisting may be true
	// even after a Stop, if the GUI was previously connected to an
	// external proxy. (The run-loop is responsible for keeping these
	// in sync, but the storage layer must not impose a constraint.)
	s := newTestServer()
	s.SetProxyRunning(false)
	s.SetConnectedToExisting(true)

	resp := decodeMetricsResponse(t, s)
	if resp.ProxyRunning {
		t.Error("ProxyRunning = true, want false")
	}
	if !resp.ConnectedExisting {
		t.Error("ConnectedExisting = false, want true")
	}
}

func TestHandleHistory_NilHistory_ReturnsEmptyArray(t *testing.T) {
	// When history is nil we must still respond with a JSON array,
	// never `null`. The frontend does allHistory = (await r.json()) || [].
	s := newTestServer() // s.hist == nil
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	s.handleHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", rec.Code, rec.Body.String())
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("body = %q, want \"[]\"", body)
	}
}

func TestSecurityHeadersMiddleware_SetsHeaders(t *testing.T) {
	h := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want \"nosniff\"", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("Content-Security-Policy not set")
	}
}

func TestSecurityHeadersMiddleware_PassesThrough(t *testing.T) {
	called := false
	h := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)
	if !called {
		t.Error("downstream handler was not called")
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", rec.Code)
	}
}

func TestWriteJSON_SetsContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, map[string]any{"ok": true})

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Errorf("body = %q, want contains `\"ok\":true`", rec.Body.String())
	}
}

func TestGetProxyPort_FallsBackToStatic(t *testing.T) {
	// When atomicCfg is nil, the static proxyPort field is used.
	s := &Server{proxyPort: 8765}
	if got := s.getProxyPort(); got != 8765 {
		t.Errorf("getProxyPort() = %d, want 8765", got)
	}
}
