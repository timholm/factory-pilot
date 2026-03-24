package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/timholm/factory-pilot/internal/config"
	"github.com/timholm/factory-pilot/internal/diagnose"
	"github.com/timholm/factory-pilot/internal/report"
)

// Server provides HTTP monitoring endpoints.
type Server struct {
	cfg       *config.Config
	collector *diagnose.Collector
	reporter  *report.Reporter
	mux       *http.ServeMux
}

// NewServer creates an API server.
func NewServer(cfg *config.Config, collector *diagnose.Collector, reporter *report.Reporter) *Server {
	s := &Server{
		cfg:       cfg,
		collector: collector,
		reporter:  reporter,
		mux:       http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /status", s.handleStatus)
	s.mux.HandleFunc("GET /reports", s.handleReports)
	s.mux.HandleFunc("GET /reports/", s.handleReportByDate)
	s.mux.HandleFunc("GET /issues", s.handleIssues)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := ":" + s.cfg.APIPort
	log.Printf("[api] listening on %s", addr)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	status := s.collector.Collect(ctx)
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	reports, err := s.reporter.GetDB().ListRecent(ctx, 30)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, reports)
}

func (s *Server) handleReportByDate(w http.ResponseWriter, r *http.Request) {
	// Extract date from /reports/2024-01-15
	date := strings.TrimPrefix(r.URL.Path, "/reports/")
	if date == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "date required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rep, err := s.reporter.GetDB().GetByDate(ctx, date)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (s *Server) handleIssues(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	issuesJSON, err := s.reporter.GetDB().GetCurrentIssues(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(issuesJSON)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
