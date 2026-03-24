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

func TestHealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		PostgresURL:    "postgres://localhost:5432/test",
		FactoryDataDir: "/tmp/factory",
		FactoryGitDir:  "/tmp/repos",
		K8sNamespace:   "factory",
		GithubUser:     "testuser",
	}

	collector := diagnose.NewCollector(cfg)
	reporter := report.NewReporter(cfg)
	server := NewServer(cfg, collector, reporter)

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

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json content type, got %q", w.Header().Get("Content-Type"))
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
