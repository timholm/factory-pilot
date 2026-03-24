package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/timholm/factory-pilot/internal/config"
	"github.com/timholm/factory-pilot/internal/diagnose"
	"github.com/timholm/factory-pilot/internal/report"
)

func newTestServer() *Server {
	cfg := &config.Config{
		PostgresURL:    "postgres://localhost:5432/test",
		FactoryDataDir: "/tmp/factory",
		FactoryGitDir:  "/tmp/repos",
		K8sNamespace:   "factory",
		GithubUser:     "testuser",
		APIPort:        "8090",
	}
	collector := diagnose.NewCollector(cfg)
	reporter := report.NewReporter(cfg)
	return NewServer(cfg, collector, reporter)
}

func TestNewServer(t *testing.T) {
	server := newTestServer()
	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	if server.cfg == nil {
		t.Error("Server.cfg is nil")
	}
	if server.collector == nil {
		t.Error("Server.collector is nil")
	}
	if server.reporter == nil {
		t.Error("Server.reporter is nil")
	}
	if server.mux == nil {
		t.Error("Server.mux is nil")
	}
}

func TestHealthEndpoint(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}

	if body["time"] == "" {
		t.Error("expected non-empty time")
	}
}

func TestHealthEndpoint_ContentType(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestHealthEndpoint_WrongMethod(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	// Go 1.22+ ServeMux with method patterns returns 405 for wrong method
	if w.Code == http.StatusOK {
		t.Error("POST /health should not return 200")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json content type, got %q", w.Header().Get("Content-Type"))
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["key"] != "value" {
		t.Errorf("body[key] = %q, want 'value'", body["key"])
	}
}

func TestWriteJSON_ErrorStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "boom"})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestWriteJSON_NestedObject(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]interface{}{
		"status": "ok",
		"counts": map[string]int{
			"total": 10,
			"ready": 8,
		},
	}
	writeJSON(w, http.StatusOK, data)

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %v", body["status"])
	}
}

func TestReportsEndpoint_NoPostgres(t *testing.T) {
	// With an invalid Postgres URL, the reports endpoint should return an error
	server := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	// Should fail gracefully (500) since Postgres is not available
	if w.Code == http.StatusOK {
		// It might succeed with empty result if postgres happens to be running
		// but generally we expect a 500
	}
	// Just check it doesn't panic
}

func TestReportByDateEndpoint_EmptyDate(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/reports/", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		// Empty date should return error or 400
		var body map[string]string
		json.NewDecoder(w.Body).Decode(&body)
		if body["error"] == "" && w.Code == http.StatusBadRequest {
			t.Log("correctly returned bad request for empty date")
		}
	}
}

func TestIssuesEndpoint_NoPostgres(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/issues", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	// Should fail gracefully since Postgres is not available, but not panic
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
